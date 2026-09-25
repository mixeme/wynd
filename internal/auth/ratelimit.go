package auth

import "errors"

// RateLimitError carries how long to wait before another code request.
type RateLimitError struct {
	RetryAfterSec int
}

// Error reports the rate limit; the wait hint is in RetryAfterSec.
func (e *RateLimitError) Error() string {
	return ErrRateLimited.Error()
}

// Unwrap lets errors.Is match ErrRateLimited.
func (e *RateLimitError) Unwrap() error {
	return ErrRateLimited
}

// RetryAfterSeconds returns the wait hint for a rate-limit response.
func RetryAfterSeconds(err error) int {
	var rl *RateLimitError
	if errors.As(err, &rl) {
		return rl.RetryAfterSec
	}
	return 60
}
