package parser

import (
	"testing"
)

func BenchmarkParseLine(b *testing.B) {
	p := NewParser()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = p.ParseLine("kernel: eth0: link down")
	}
}

func BenchmarkSanitizeLog(b *testing.B) {
	p := NewParser()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = p.SanitizeLog("kernel: test; rm -rf / | reboot & echo $<>\\")
	}
}
