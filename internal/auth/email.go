package auth

import (
	"net/mail"
	"strings"
)

// ParseParticipantEmail trims input, parses a mailbox with net/mail, and
// returns a normalized address. Local part and domain must be non-empty and
// the domain must contain a dot.
func ParseParticipantEmail(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", ErrInvalid
	}
	parsed, err := mail.ParseAddress(raw)
	if err != nil {
		return "", ErrInvalid
	}
	email := normalizeEmail(parsed.Address)
	local, domain, ok := strings.Cut(email, "@")
	if !ok || local == "" || domain == "" || !strings.Contains(domain, ".") {
		return "", ErrInvalid
	}
	return email, nil
}
