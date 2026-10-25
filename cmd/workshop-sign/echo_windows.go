package main

import (
	"os"

	"golang.org/x/sys/windows"
)

// noEcho выключает показ вводимых знаков в консоли Windows и возвращает,
// чем его включить обратно. Ввод не из консоли — ничего не делает.
func noEcho() func() {
	h := windows.Handle(os.Stdin.Fd())
	var mode uint32
	if windows.GetConsoleMode(h, &mode) != nil {
		return func() {}
	}
	if windows.SetConsoleMode(h, mode&^windows.ENABLE_ECHO_INPUT) != nil {
		return func() {}
	}
	return func() { _ = windows.SetConsoleMode(h, mode) }
}
