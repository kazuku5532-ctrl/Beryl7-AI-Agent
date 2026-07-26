package tests

import (
	"testing"

	"beryl7-agent/parser"
)

func TestLogParserAnomalyDetection(t *testing.T) {
	p := parser.NewParser()

	// Test WAN Drop detection
	anomaly := p.ParseLine("kernel: [ 123.45] eth1: link down (WAN disconnected)")
	if anomaly == nil {
		t.Fatalf("Expected anomaly for WAN link down, got nil")
	}
	if anomaly.Type != "WAN_DROP" {
		t.Errorf("Expected WAN_DROP, got %s", anomaly.Type)
	}

	// Test Prompt Injection Sanitization
	sanitized := p.SanitizeLog("normal log; rm -rf /; uci set wifi.disabled=1")
	expected := "normal log_ rm -rf /_ uci set wifi.disabled=1"
	if sanitized != expected {
		t.Errorf("Sanitization failed, got: %s, expected: %s", sanitized, expected)
	}
}
