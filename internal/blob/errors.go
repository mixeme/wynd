package blob

import (
	"errors"
	"fmt"
)

var (
	ErrNotFound      = errors.New("blob: not found")
	ErrForbidden     = errors.New("blob: forbidden")
	ErrInvalid       = errors.New("blob: invalid")
	ErrQuotaExceeded = errors.New("blob: quota exceeded")
	// ErrInstanceQuotaExceeded — упёрся потолок сервера, а не квота круга
	// (3.11): писать нельзя ни в один круг. Is(ErrQuotaExceeded) тоже верно.
	ErrInstanceQuotaExceeded = fmt.Errorf("%w: instance", ErrQuotaExceeded)
	ErrIncomplete            = errors.New("blob: incomplete")
	ErrExpired               = errors.New("blob: expired")
)
