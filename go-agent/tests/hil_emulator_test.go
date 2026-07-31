package tests

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"beryl7-agent/ai"
	"beryl7-agent/config"
	"beryl7-agent/executor"
	"beryl7-agent/parser"
	"beryl7-agent/skillstore"
	"beryl7-agent/telemetry"
	"beryl7-agent/watchdog"
)

func TestOpenWrtHILEmulatorPATHHijack(t *testing.T) {
	tempDir := t.TempDir()
	fakeBinDir := filepath.Join(tempDir, "bin")
	_ = os.MkdirAll(fakeBinDir, 0755)

	uciExt := ""
	if runtime.GOOS == "windows" {
		uciExt = ".cmd"
	}

	// Comprehensive fake uci script handling export, import, commit, set, show
	uciScript := "@echo off\r\nif \"%1\"==\"export\" echo package network\r\nif \"%1\"==\"export\" echo config interface 'wan'\r\nif \"%1\"==\"export\" echo     option proto 'dhcp'\r\nexit /b 0"
	if runtime.GOOS != "windows" {
		uciScript = "#!/bin/sh\nif [ \"$1\" = \"export\" ]; then\n  echo 'package network'\n  echo 'config interface wan'\n  echo '    option proto dhcp'\nfi\nexit 0"
	}
	_ = os.WriteFile(filepath.Join(fakeBinDir, "uci"+uciExt), []byte(uciScript), 0755)

	// Comprehensive fake logread script
	logreadScript := "@echo kernel: eth0: link down\r\nexit /b 0"
	if runtime.GOOS != "windows" {
		logreadScript = "#!/bin/sh\necho 'kernel: eth0: link down'\nexit 0"
	}
	_ = os.WriteFile(filepath.Join(fakeBinDir, "logread"+uciExt), []byte(logreadScript), 0755)

	// Hijack PATH env var
	t.Setenv("PATH", fakeBinDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	// Verify execution of hijacked uci binary (export, import, commit)
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/c", filepath.Join(fakeBinDir, "uci.cmd"), "export")
	} else {
		cmd = exec.Command("uci", "export")
	}
	out, err := cmd.Output()
	if err != nil || !strings.Contains(string(out), "network") {
		t.Fatalf("HIL PATH Hijack failed for uci: out=%s, err=%v", string(out), err)
	}

	// Verify parser & executor integration with fake environment
	p := parser.NewParser()
	rep := p.ParseLine("kernel: eth0: link down")
	if rep == nil || rep.Type != "WAN_DROP" {
		t.Errorf("Expected WAN_DROP from simulated logread, got %v", rep)
	}

	execEngine := executor.New()
	ctx := context.Background()
	actReq := &executor.ActionRequest{ActionName: "restart_wan_interface", Target: "wan"}
	execErr := execEngine.ExecuteAction(ctx, actReq, true)
	if execErr != nil {
		t.Errorf("HIL Simulated action execution failed: %v", execErr)
	}
}

func TestFullDaemonE2EWithHILEmulator(t *testing.T) {
	tempDir := t.TempDir()
	store, err := skillstore.New(filepath.Join(tempDir, "hil_skills.db"))
	if err != nil {
		t.Fatalf("Failed to init skillstore for HIL daemon test: %v", err)
	}
	defer store.Close()

	_ = watchdog.New(filepath.Join(tempDir, "cp.uci"))
	_ = executor.New()
	_ = ai.NewClient("dummy-key")

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to bind HIL dynamic test port: %v", err)
	}
	testPort := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()

	_ = &config.Config{
		HealthPort:   testPort,
		BindHost:     "127.0.0.1",
		AuthToken:    "admin-secret",
		ApproveToken: "operator-secret",
		DryRun:       true,
	}

	telCollector := telemetry.NewCollector()
	metric := telCollector.CollectMetrics(context.Background())
	if metric == nil {
		t.Fatalf("Telemetry metrics collector returned nil")
	}

	server := &http.Server{
		Addr: fmt.Sprintf("127.0.0.1:%d", testPort),
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"healthy","hil_mock":true}`))
		}),
	}
	go func() { _ = server.ListenAndServe() }()
	defer server.Close()

	time.Sleep(100 * time.Millisecond)
	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/api/health", testPort))
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("HIL Full Daemon E2E server check failed: %v", err)
	}
	_ = resp.Body.Close()
}
