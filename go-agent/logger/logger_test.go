package logger

import (
	"path/filepath"
	"testing"
)

func TestLoggerInit(t *testing.T) {
	tempDir := t.TempDir()
	logFile := filepath.Join(tempDir, "test.log")

	l, err := Init(logFile, "INFO")
	if err != nil {
		t.Fatalf("Failed to init logger: %v", err)
	}

	Info("Test info message")
	Warn("Test warn message")
	Error("Test error message")
	Flush()
	l.Close()
}
