package chronicle

import "strings"

var validCircleColors = map[string]struct{}{
	"terracotta": {},
	"teal":       {},
	"olive":      {},
	"ochre":      {},
	"plum":       {},
	"indigo":     {},
	"coffee":     {},
	"slate":      {},
}

func normalizeCircleColor(color string) (string, error) {
	c := strings.TrimSpace(strings.ToLower(color))
	if c == "" {
		return "ochre", nil
	}
	if _, ok := validCircleColors[c]; !ok {
		return "", ErrInvalid
	}
	return c, nil
}
