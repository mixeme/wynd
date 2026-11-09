//go:build !windows

package desktop

import "errors"

// ErrAutostartUnsupported — автозапуск пока только Windows (план 55).
var ErrAutostartUnsupported = errors.New("autostart is only supported on Windows")

type AutostartStatus struct {
	Enabled bool   `json:"enabled"`
	Stale   bool   `json:"stale"`
	Command string `json:"command"`
}

func AutostartCommand() (string, error) { return "", ErrAutostartUnsupported }

func AutostartState() (AutostartStatus, error) { return AutostartStatus{}, nil }

func SetAutostart(bool) (AutostartStatus, error) { return AutostartStatus{}, ErrAutostartUnsupported }
