//go:build !windows

package signing

import "os/exec"

func hideWindow(*exec.Cmd) {}
