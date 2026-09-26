package push

import "errors"

var (
	ErrNotConfigured = errors.New("push: vapid not configured")
	ErrInvalid       = errors.New("push: invalid")
	ErrNotFound      = errors.New("push: not found")
	// ErrForbidden: the endpoint belongs to another account and the caller
	// did not present that subscription's keys.
	ErrForbidden = errors.New("push: forbidden")
	// ErrNoSubscriptions: the account has no device to deliver to. Only the
	// test push reports it — ordinary signals to an unsubscribed member are
	// not an error.
	ErrNoSubscriptions = errors.New("push: no subscriptions")
	// ErrDelivery wraps a failed delivery so the API can tell it from a bug.
	ErrDelivery = errors.New("push: delivery failed")
)
