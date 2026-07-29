package main

import (
	"testing"
)

func TestGetSystemLogSample(t *testing.T) {
	sample := getSystemLogSample()
	t.Logf("System log sample output len: %d", len(sample))
}
