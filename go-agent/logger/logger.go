package logger

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"
)

type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
)

type Logger struct {
	mu          sync.Mutex
	level       LogLevel
	file        *os.File
	filePath    string
	maxBytes    int64
	backupCount int
	jsonMode    bool
	webhookURL  string
}

var (
	globalLogger *Logger
	ipRegex      = regexp.MustCompile(`\b\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}\b`)
	macRegex     = regexp.MustCompile(`(?i)\b([0-9A-F]{2}[:-]){5}([0-9A-F]{2})\b`)
	passRegex    = regexp.MustCompile(`(?i)(password|secret|key|token|auth)=["']?([^"' \n]+)["']?`)
	bearerRegex  = regexp.MustCompile(`(?i)Bearer\s+[A-Za-z0-9\-\._~\+\/]+=*`)
	jwtRegex     = regexp.MustCompile(`eyJ[A-Za-z0-9\-_%]+\.eyJ[A-Za-z0-9\-_%]+\.[A-Za-z0-9\-_%]+`)
)

func Init(filePath string, levelStr string) (*Logger, error) {
	var lvl LogLevel
	switch levelStr {
	case "DEBUG":
		lvl = DEBUG
	case "WARN":
		lvl = WARN
	case "ERROR":
		lvl = ERROR
	default:
		lvl = INFO
	}

	l := &Logger{
		level:       lvl,
		filePath:    filePath,
		maxBytes:    2 * 1024 * 1024,
		backupCount: 5,
	}

	if filePath != "" {
		cleanPath := filepath.Clean(filePath)
		f, err := os.OpenFile(cleanPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600) // #nosec G304
		if err == nil {
			l.file = f
		}
	}

	globalLogger = l
	return l, nil
}

func (l *Logger) SetJSONMode(enabled bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.jsonMode = enabled
}

func (l *Logger) SetWebhookURL(url string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.webhookURL = url
}

func (l *Logger) rotate() {
	if l.file == nil {
		return
	}

	fi, err := l.file.Stat()
	if err != nil || fi.Size() < l.maxBytes {
		return
	}

	_ = l.file.Close()
	maxBackups := l.backupCount
	if maxBackups <= 0 {
		maxBackups = 5
	}

	for i := maxBackups; i >= 1; i-- {
		oldPath := fmt.Sprintf("%s.%d", l.filePath, i)
		if i == maxBackups {
			_ = os.Remove(oldPath)
		} else {
			newPath := fmt.Sprintf("%s.%d", l.filePath, i+1)
			_ = os.Rename(oldPath, newPath)
		}
	}
	_ = os.Rename(l.filePath, l.filePath+".1")

	f, err := os.OpenFile(l.filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err == nil {
		l.file = f
	}
}

func (l *Logger) SetBackupCount(count int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if count > 0 {
		l.backupCount = count
	}
}

func sanitizeRedact(msg string) string {
	msg = passRegex.ReplaceAllString(msg, "$1=***REDACTED***")
	msg = bearerRegex.ReplaceAllString(msg, "Bearer ***REDACTED***")
	msg = jwtRegex.ReplaceAllString(msg, "***JWT_REDACTED***")
	msg = macRegex.ReplaceAllString(msg, "[REDACTED_MAC]")
	msg = ipRegex.ReplaceAllStringFunc(msg, func(ip string) string {
		parts := regexp.MustCompile(`\.`).Split(ip, -1)
		if len(parts) == 4 {
			return fmt.Sprintf("%s.%s.x.x", parts[0], parts[1])
		}
		return ip
	})
	return msg
}

func (l *Logger) log(level LogLevel, levelStr, format string, v ...interface{}) {
	if level < l.level {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	l.rotate()

	timestamp := time.Now().UTC().Format(time.RFC3339)
	msg := fmt.Sprintf(format, v...)

	msg = sanitizeRedact(msg)

	line := fmt.Sprintf("[%s] [%s] %s\n", timestamp, levelStr, msg)

	fmt.Print(line)

	if l.file != nil {
		_, _ = io.WriteString(l.file, line)
	}
}

func Debug(format string, v ...interface{}) {
	if globalLogger != nil {
		globalLogger.log(DEBUG, "DEBUG", format, v...)
	}
}

func Info(format string, v ...interface{}) {
	if globalLogger != nil {
		globalLogger.log(INFO, "INFO", format, v...)
	}
}

func Warn(format string, v ...interface{}) {
	if globalLogger != nil {
		globalLogger.log(WARN, "WARN", format, v...)
	}
}

func Error(format string, v ...interface{}) {
	if globalLogger != nil {
		globalLogger.log(ERROR, "ERROR", format, v...)
	}
}

func Fatal(format string, v ...interface{}) {
	if globalLogger != nil {
		globalLogger.log(ERROR, "FATAL", format, v...)
		globalLogger.Flush()
	}
	os.Exit(1)
}

func (l *Logger) Flush() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file != nil {
		_ = l.file.Sync()
	}
}

func (l *Logger) Close() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file != nil {
		_ = l.file.Close()
		l.file = nil
	}
}

func Flush() {
	if globalLogger != nil {
		globalLogger.Flush()
	}
}

func Close() {
	if globalLogger != nil {
		globalLogger.Close()
	}
}
