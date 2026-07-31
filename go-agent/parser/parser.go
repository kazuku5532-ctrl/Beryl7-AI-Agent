package parser

import (
	"regexp"
	"strings"
	"sync"
	"time"

	"beryl7-agent/logger"
)

type Anomaly struct {
	Type        string    `json:"type"`
	Severity    string    `json:"severity"`
	SourceLog   string    `json:"source_log"`
	DetectedAt  time.Time `json:"detected_at"`
	Description string    `json:"description"`
}

type LogParser struct {
	mu             sync.Mutex
	rateLimiter    int
	lastReset      time.Time
	maxLinesPerSec int
	bufferPool     sync.Pool
	wanDropRegex   *regexp.Regexp
	wifiFailRegex  *regexp.Regexp
	memLimitRegex  *regexp.Regexp
	metaCharRegex  *regexp.Regexp
}

func NewParser() *LogParser {
	return &LogParser{
		maxLinesPerSec: 100, // Tối đa 100 dòng log/giây chống tràn log flood attack
		lastReset:      time.Now(),
		bufferPool: sync.Pool{
			New: func() interface{} {
				b := make([]byte, 4096) // 4096 bytes max syslog line
				return &b
			},
		},
		wanDropRegex:  regexp.MustCompile(`(?i)(link\s+down|wan\s+disconnected|dhcp\s+failed|carrier\s+lost)`),
		wifiFailRegex: regexp.MustCompile(`(?i)(beacon\s+loss|hostapd.*failed|wlan.*deauth|auth\s+timeout)`),
		memLimitRegex: regexp.MustCompile(`(?i)(out\s+of\s+memory|oom-killer|page\s+allocation\s+failure)`),
		metaCharRegex: regexp.MustCompile(`[;&|` + "`" + `$<>\\]`), // Prompt Injection Sanitizer
	}
}

// SanitizeLog Line lọc sạch các ký tự Shell & SQL Meta-characters nguy hiểm
func (p *LogParser) SanitizeLog(line string) string {
	if len(line) > 4096 {
		line = line[:4096] // 4096 bytes max syslog truncation
	}
	return p.metaCharRegex.ReplaceAllString(line, "_")
}

// ParseLine phân tích 1 dòng log, bọc rate limiter chống tràn CPU
func (p *LogParser) ParseLine(rawLine string) *Anomaly {
	p.mu.Lock()
	now := time.Now()
	if now.Sub(p.lastReset) >= time.Second {
		p.rateLimiter = 0
		p.lastReset = now
	}

	if p.rateLimiter >= p.maxLinesPerSec {
		p.mu.Unlock()
		return nil // Bỏ qua nếu tràn rate limit log
	}
	p.rateLimiter++
	p.mu.Unlock()

	line := p.SanitizeLog(rawLine)
	if strings.TrimSpace(line) == "" {
		return nil
	}

	// Tái sử dụng Buffer Pool tránh ép tải cho Go Garbage Collector trên chip nhúng
	bufPtr := p.bufferPool.Get().(*[]byte)
	defer p.bufferPool.Put(bufPtr)

	if p.wanDropRegex.MatchString(line) {
		logger.Warn("Logread Anomaly Detected: WAN Drop -> %s", line)
		return &Anomaly{
			Type:        "WAN_DROP",
			Severity:    "HIGH",
			SourceLog:   line,
			DetectedAt:  time.Now().UTC(),
			Description: "WAN interface disconnection or link down event detected in syslog",
		}
	}

	if p.wifiFailRegex.MatchString(line) {
		logger.Warn("Logread Anomaly Detected: Wi-Fi Failure -> %s", line)
		return &Anomaly{
			Type:        "WIFI_FAILURE",
			Severity:    "MEDIUM",
			SourceLog:   line,
			DetectedAt:  time.Now().UTC(),
			Description: "Wi-Fi beacon loss or hostapd deauthentication detected in syslog",
		}
	}

	if p.memLimitRegex.MatchString(line) {
		logger.Warn("Logread Anomaly Detected: OOM Memory Exhaustion -> %s", line)
		return &Anomaly{
			Type:        "MEMORY_EXHAUSTION",
			Severity:    "CRITICAL",
			SourceLog:   line,
			DetectedAt:  time.Now().UTC(),
			Description: "Kernel Out-Of-Memory (OOM) allocation failure detected in syslog",
		}
	}

	return nil
}
