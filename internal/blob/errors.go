package blob

import (
	"errors"
)

var (
	ErrNotFound      = errors.New("blob: not found")
	ErrForbidden     = errors.New("blob: forbidden")
	ErrInvalid       = errors.New("blob: invalid")
	ErrQuotaExceeded = errors.New("blob: quota exceeded")
	ErrIncomplete    = errors.New("blob: incomplete")
	ErrExpired       = errors.New("blob: expired")
)
