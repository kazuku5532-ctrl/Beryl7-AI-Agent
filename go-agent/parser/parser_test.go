package parser

import (
	"strings"
	"testing"
	"time"
)

func TestParserAllBranches(t *testing.T) {
	p := NewParser()

	// 1. Sanitizer meta-characters
	unsafe := "kernel: test; rm -rf / | reboot & echo $<>\\"
	sanitized := p.SanitizeLog(unsafe)
	if strings.Contains(sanitized, ";") || strings.Contains(sanitized, "|") {
		t.Errorf("Sanitizer failed to redact meta-characters: %s", sanitized)
	}

	// 2. Long line truncation (> 4096 bytes)
	longLine := strings.Repeat("A", 5000)
	truncated := p.SanitizeLog(longLine)
	if len(truncated) > 4096 {
		t.Errorf("Expected line truncation to 4096 bytes, got %d", len(truncated))
	}

	// 3. WAN Drop
	rep1 := p.ParseLine("kernel: eth0: link down")
	if rep1 == nil || rep1.Type != "WAN_DROP" {
		t.Errorf("Expected WAN_DROP report, got %v", rep1)
	}

	// 4. Wi-Fi Failure
	rep2 := p.ParseLine("hostapd: wlan0: hostapd failed beacon loss")
	if rep2 == nil || rep2.Type != "WIFI_FAILURE" {
		t.Errorf("Expected WIFI_FAILURE report, got %v", rep2)
	}

	// 5. Memory Exhaustion
	rep3 := p.ParseLine("kernel: Out of memory: oom-killer invoked")
	if rep3 == nil || rep3.Type != "MEMORY_EXHAUSTION" {
		t.Errorf("Expected MEMORY_EXHAUSTION report, got %v", rep3)
	}

	// 6. Normal line
	rep4 := p.ParseLine("dnsmasq[123]: query[A] google.com from 192.168.8.100")
	if rep4 != nil {
		t.Errorf("Expected nil for normal log, got %v", rep4)
	}

	// 7. Empty line
	rep5 := p.ParseLine("   \n  ")
	if rep5 != nil {
		t.Errorf("Expected nil for empty line, got %v", rep5)
	}

	// 8. Rate limiter flood test (> 100 lines/sec)
	p.lastReset = time.Now()
	p.rateLimiter = 100
	rep6 := p.ParseLine("kernel: eth0: link down")
	if rep6 != nil {
		t.Errorf("Expected nil when rate limit exceeded, got %v", rep6)
	}

	p.lastReset = time.Now().Add(-2 * time.Second)
	rep7 := p.ParseLine("kernel: eth0: link down")
	if rep7 == nil {
		t.Errorf("Expected rate limiter reset after 1 sec")
	}
}
