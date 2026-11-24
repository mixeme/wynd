//go:build !windows && !linux

package desktop

// Схема wynd: живёт в scheme_windows.go (реестр Windows); заглушка —
// серверным тестам на Linux.

func EnsureInviteScheme() error { return nil }

func InviteSchemeCommand() (string, error) { return "", nil }
