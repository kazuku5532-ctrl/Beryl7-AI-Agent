package parser

import (
	"testing"
)

func TestParserLineMatching(t *testing.T) {
	p := NewParser()

	tests := []struct {
		line        string
		expectedTyp string
	}{
		{"kernel: eth0: link down", "WAN_DROP"},
		{"hostapd: wlan0: hostapd failed", "WIFI_FAILURE"},
		{"kernel: Out of memory: Kill process 123", "MEMORY_EXHAUSTION"},
		{"normal log message nothing special", ""},
	}

	for _, tt := range tests {
		report := p.ParseLine(tt.line)
		if tt.expectedTyp == "" {
			if report != nil {
				t.Errorf("Expected nil report for line '%s', got %v", tt.line, report.Type)
			}
		} else {
			if report == nil {
				t.Errorf("Expected report type %s for line '%s', got nil", tt.expectedTyp, tt.line)
			} else if report.Type != tt.expectedTyp {
				t.Errorf("Expected report type %s, got %s", tt.expectedTyp, report.Type)
			}
		}
	}
}

func TestSanitizeLog(t *testing.T) {
	p := NewParser()
	raw := "line1\n\n  \nline2\n"
	sanitized := p.SanitizeLog(raw)
	if sanitized == "" {
		t.Errorf("Expected non-empty sanitized log")
	}
}
