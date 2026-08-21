package logger

import (
	"path/filepath"
	"testing"
)

func TestLoggerInit(t *testing.T) {
	tempDir := t.TempDir()
	logFile := filepath.Join(tempDir, "test.log")

	l, err := Init(logFile, "DEBUG")
	if err != nil {
		t.Fatalf("Failed to init logger: %v", err)
	}

	l.SetJSONMode(true)
	l.SetWebhookURL("http://127.0.0.1:9999/webhook")
	l.SetBackupCount(3)
	l.SetMaxBytes(1024 * 1024)

	Debug("Test debug message with key AIzaSyA8hjVP9iwhRCPACZdhLSBNcuOItKAH3Qs")
	Info("Test info message")
	Warn("Test warn message")
	Error("Test error message")
	Flush()
	l.Close()
}
