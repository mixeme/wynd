package blob

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestSanitizeFilenameTruncatesByRunes(t *testing.T) {
	name := strings.Repeat("я", maxFilenameLen+10)
	got := sanitizeFilename(name)
	if len([]rune(got)) != maxFilenameLen {
		t.Fatalf("expected %d runes, got %d", maxFilenameLen, len([]rune(got)))
	}
	if !utf8.ValidString(got) {
		t.Fatal("truncated name is not valid UTF-8")
	}
}
