package chronicle

import "context"

// Точечные запросы к проекции: счётчики и разрешение имён. Лежали в
// day.go рядом с логикой дня, хотя ко дню отношения не имеют (RDB-2).

// CountCirclePosts counts non-deleted posts in a circle.
func (c *Chronicle) CountCirclePosts(ctx context.Context, circleID string) (int, error) {
	var n int
	err := c.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM posts WHERE circle_id = ? AND deleted = 0
	`, circleID).Scan(&n)
	return n, err
}

// ResolveIdentityName returns current name or empty if erased.
func (c *Chronicle) ResolveIdentityName(ctx context.Context, identityID string) (string, error) {
	return c.identityName(ctx, c.db, identityID)
}
