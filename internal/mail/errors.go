package mail

import "errors"

var (
	ErrNotConfigured = errors.New("mail: smtp not configured")
	ErrInvalid       = errors.New("mail: invalid")
)
