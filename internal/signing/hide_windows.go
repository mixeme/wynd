//go:build windows

package signing

import (
	"os/exec"
	"syscall"
)

// hideWindow — gh из окна не открывает чёрное окно консоли.
func hideWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000} // CREATE_NO_WINDOW
}
