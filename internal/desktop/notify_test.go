package desktop

import (
	"encoding/binary"
	"os"
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

var pngLayer = []byte{0x89, 0x50, 0x4E, 0x47, 0, 0, 0, 0}

// makeIco собирает .ico из слоёв; пустые слои — не PNG.
func makeIco(layers ...[]byte) []byte {
	buf := []byte{0, 0, 1, 0, byte(len(layers)), 0}
	off := 6 + 16*len(layers)
	for _, l := range layers {
		buf = append(buf, 0, 0, 0, 0, 1, 0, 32, 0)
		var hdr [8]byte
		binary.LittleEndian.PutUint32(hdr[0:4], uint32(len(l)))
		binary.LittleEndian.PutUint32(hdr[4:8], uint32(off))
		buf = append(buf, hdr[:]...)
		off += len(l)
	}
	for _, l := range layers {
		buf = append(buf, l...)
	}
	return buf
}

func TestExtractIconPNG(t *testing.T) {
	small := append([]byte(nil), pngLayer...)
	big := append(append([]byte(nil), pngLayer...), 1, 2, 3, 4)
	got, err := extractIconPNG(makeIco(small, big))
	if err != nil {
		t.Fatalf("PNG-слой не найден: %v", err)
	}
	if !bytesEqualPrefix(got, pngLayer) || len(got) != len(big) {
		t.Errorf("взят не самый крупный слой: %d байт", len(got))
	}
	if _, err := extractIconPNG(makeIco([]byte{0, 0, 0, 0, 0, 0, 0, 0})); err == nil {
		t.Error("слой без PNG-подписи принят")
	}
	if _, err := extractIconPNG([]byte("мусор")); err == nil {
		t.Error("не-ICO принят")
	}
}

// Настоящий значок окна обязан содержать PNG-слой: без него уведомления
// молча теряют значок (см. EnsureToastIdentity).
func TestDesktopIconHasPNGLayer(t *testing.T) {
	ico, err := os.ReadFile("../../cmd/wynd-desktop/build/windows/icon.ico")
	if err != nil {
		t.Skipf("значок окна недоступен из этого контура: %v", err)
	}
	if _, err := extractIconPNG(ico); err != nil {
		t.Errorf("в build/windows/icon.ico нет PNG-слоя: %v", err)
	}
}

func bytesEqualPrefix(b, prefix []byte) bool {
	return len(b) >= len(prefix) && string(b[:len(prefix)]) == string(prefix)
}
