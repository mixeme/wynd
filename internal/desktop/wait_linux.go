//go:build linux

package desktop

import (
	"errors"
	"syscall"
	"time"
)

// WaitForProcessExit ждёт выхода прежнего процесса обновления: пока он жив,
// Wails отдаёт новому запуску управление и новый закрывается. timeout —
// предохранитель, дальше обычный запуск.
func WaitForProcessExit(pid int, timeout time.Duration) bool {
	if pid <= 0 {
		return true
	}
	deadline := time.Now().Add(timeout)
	for {
		err := syscall.Kill(pid, 0)
		if errors.Is(err, syscall.ESRCH) {
			return true
		}
		if err != nil && !errors.Is(err, syscall.EPERM) {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(50 * time.Millisecond)
	}
}
