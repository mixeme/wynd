package desktop

import (
	"testing"
)

func TestShouldNotify(t *testing.T) {
	cases := []struct {
		name               string
		visible, minimised bool
		want               bool
	}{
		{"на виду", true, false, false},
		{"свёрнуто", true, true, true},
		{"спрятано в трей", false, false, true},
		{"спрятано и свёрнуто", false, true, true},
	}
	for _, c := range cases {
		if got := ShouldNotify(c.visible, c.minimised); got != c.want {
			t.Errorf("%s: ShouldNotify(%v, %v) = %v, ждём %v", c.name, c.visible, c.minimised, got, c.want)
		}
	}
}

func TestSanitizeToastText(t *testing.T) {
	if got := sanitizeToastText("a]]>b"); got != "a]]&gt;b" {
		t.Errorf("«]]>» не обезврежена: %q", got)
	}
	if got := sanitizeToastText("обычный текст"); got != "обычный текст" {
		t.Errorf("обычный текст изменён: %q", got)
	}
}
