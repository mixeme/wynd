package auth

import "errors"

var (
	ErrNotFound        = errors.New("auth: not found")
	ErrConflict        = errors.New("auth: conflict")
	ErrForbidden       = errors.New("auth: forbidden")
	ErrInvalid         = errors.New("auth: invalid")
	ErrRateLimited     = errors.New("auth: rate limited")
	ErrTooManyAttempts = errors.New("auth: too many attempts")
	ErrExpired         = errors.New("auth: expired")
	ErrClosed          = errors.New("auth: registration closed")
	ErrPaymentRequired = errors.New("auth: payment required")
)

// ErrWeakPassword is returned when an admin password is shorter than MinPasswordLen.
var ErrWeakPassword = errors.New("auth: weak password")
