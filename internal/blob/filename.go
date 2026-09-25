package blob

import (
	"path/filepath"
	"strings"
)

const maxFilenameLen = 255

// sanitizeFilename keeps a safe display/download name from client input.
func sanitizeFilename(name string) string {
	name = filepath.Base(strings.TrimSpace(name))
	name = strings.Map(func(r rune) rune {
		if r == 0 || r < 32 {
			return -1
		}
		return r
	}, name)
	runes := []rune(name)
	if len(runes) > maxFilenameLen {
		name = string(runes[:maxFilenameLen])
	}
	return name
}
