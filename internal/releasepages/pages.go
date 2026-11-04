// Package releasepages bounds collection of GitHub release pages for both
// Workshop self-update and its signing helper.
package releasepages

import (
	"context"
	"fmt"
)

const Size = 100
const Max = 20

// Collect never returns a partial list as a successful complete snapshot.
func Collect[T any](ctx context.Context, fetch func(int) ([]T, bool, error)) ([]T, error) {
	var result []T
	for page := 1; page <= Max; page++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		items, more, err := fetch(page)
		if err != nil {
			return nil, fmt.Errorf("release page %d: %w", page, err)
		}
		result = append(result, items...)
		if !more {
			return result, nil
		}
	}
	return nil, fmt.Errorf("release listing exceeds %d pages", Max)
}
