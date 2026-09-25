package uid

import (
	"fmt"

	"github.com/google/uuid"
)

// NewID returns a UUID v7 string.
func NewID() (string, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return "", fmt.Errorf("uid: %w", err)
	}
	return id.String(), nil
}
