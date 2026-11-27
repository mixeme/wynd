//go:build !linux

package desktop

import "time"

// WaitForProcessExit вне Linux не нужен: у Windows ожидание решает признак
// .old рядом с exe (FreshlyReplaced).
func WaitForProcessExit(_ int, _ time.Duration) bool { return true }
