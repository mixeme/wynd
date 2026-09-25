package push

import "errors"

var (
	ErrNotConfigured = errors.New("push: vapid not configured")
	ErrInvalid       = errors.New("push: invalid")
	ErrNotFound      = errors.New("push: not found")
	// ErrForbidden: the endpoint belongs to another account and the caller
	// did not present that subscription's keys.
	ErrForbidden = errors.New("push: forbidden")
)
