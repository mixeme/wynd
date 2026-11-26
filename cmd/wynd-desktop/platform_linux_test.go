//go:build linux && (dev || desktop || bindings)

package main

import "testing"

// Без значка в трее окно по крестику закрывается, а не прячется: спрятать
// его некуда, и вернуть окно было бы нечем (план 69, этап 2.6). Ветка без
// StatusNotifierWatcher живьём на машине с треем не проверяется, поэтому
// политика закреплена тестом.
func TestNoTrayCannotHide(t *testing.T) {
	if platformCanHideToTray(nil) {
		t.Fatal("без значка в трее окно прятать нельзя")
	}
}
