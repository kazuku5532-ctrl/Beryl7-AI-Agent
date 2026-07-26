//go:build windows

package main

func setOOMScore() {
	// No-op on Windows
}

func checkPIDAlive(pid int) bool {
	// Safe fallback on Windows
	return false
}

func setSoReuseAddr(fd uintptr) error {
	// Safe fallback on Windows
	return nil
}
