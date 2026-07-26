//go:build linux

package main

import (
	"os"
	"syscall"
)

func setOOMScore() {
	// G306 Fix: Đặt quyền 0600 chuẩn hóa an ninh mạng theo quy tắc Gosec
	_ = os.WriteFile("/proc/self/oom_score_adj", []byte("-500"), 0600)
}

func checkPIDAlive(pid int) bool {
	return syscall.Kill(pid, 0) == nil
}

func setSoReuseAddr(fd uintptr) error {
	return syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)
}

func parsePIDFileError(err error) bool {
	return err == syscall.ESRCH
}
