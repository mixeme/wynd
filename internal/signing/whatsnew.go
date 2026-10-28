package signing

import "strings"

// WhatsNew — раздел версии из cmd/wynd-installer/whatsnew.md: строки «- …»
// под заголовком «## <версия>», до следующего заголовка. Нет раздела — пусто.
func WhatsNew(md, version string) string {
	var out []string
	in := false
	for _, line := range strings.Split(strings.ReplaceAll(md, "\r\n", "\n"), "\n") {
		if rest, ok := strings.CutPrefix(line, "## "); ok {
			if in {
				break
			}
			in = strings.TrimSpace(rest) == version
			continue
		}
		if in && strings.TrimSpace(line) != "" {
			out = append(out, strings.TrimRight(line, " \t"))
		}
	}
	return strings.Join(out, "\n")
}
