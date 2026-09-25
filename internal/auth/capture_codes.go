package auth

import (
	"context"
	"sync"
)

// CaptureCodes stores the last sent code per email for tests.
type CaptureCodes struct {
	mu     sync.Mutex
	codes  map[string]string
	latest string
}

func NewCaptureCodes() *CaptureCodes {
	return &CaptureCodes{codes: make(map[string]string)}
}

func (c *CaptureCodes) SendCode(_ context.Context, email, code string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.codes[email] = code
	c.latest = code
	return nil
}

func (c *CaptureCodes) Last(email string) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.codes[email]
}

func (c *CaptureCodes) Latest() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.latest
}
