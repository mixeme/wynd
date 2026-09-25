package chronicle

import (
	"context"
	"sort"
	"strings"
)

// MentionedAccountIDs returns accounts mentioned by @name in body (active members only).
func (c *Chronicle) MentionedAccountIDs(ctx context.Context, circleID, body string) ([]string, error) {
	if body == "" {
		return nil, nil
	}
	names, err := c.activeMemberNames(ctx, circleID)
	if err != nil {
		return nil, err
	}
	if len(names) == 0 {
		return nil, nil
	}
	mentioned := findMentionedNames(body, names)
	if len(mentioned) == 0 {
		return nil, nil
	}
	seen := make(map[string]bool)
	var out []string
	for _, name := range mentioned {
		id := names[name]
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out, nil
}

func findMentionedNames(body string, names map[string]string) []string {
	sorted := make([]string, 0, len(names))
	for name := range names {
		sorted = append(sorted, name)
	}
	sort.Slice(sorted, func(i, j int) bool {
		return len(sorted[i]) > len(sorted[j])
	})

	var found []string
	seen := make(map[string]bool)
	for i := 0; i < len(body); i++ {
		if body[i] != '@' {
			continue
		}
		if i > 0 && !isMentionBoundary(body[i-1]) {
			continue
		}
		rest := body[i+1:]
		for _, name := range sorted {
			if name == "" {
				continue
			}
			if !strings.HasPrefix(rest, name) {
				continue
			}
			end := i + 1 + len(name)
			if end < len(body) && !isMentionBoundary(body[end]) {
				continue
			}
			if !seen[name] {
				seen[name] = true
				found = append(found, name)
			}
			break
		}
	}
	return found
}

func isMentionBoundary(ch byte) bool {
	return ch == ' ' || ch == '\n' || ch == '\r' || ch == '\t' || ch == ',' || ch == '.' || ch == '!' || ch == '?' || ch == ';' || ch == ':'
}

func (c *Chronicle) activeMemberNames(ctx context.Context, circleID string) (map[string]string, error) {
	rows, err := c.db.QueryContext(ctx, `
		SELECT m.account_id, m.identity_id
		FROM memberships m
		WHERE m.circle_id = ? AND m.status = ?
	`, circleID, StatusActive)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	names := make(map[string]string)
	for rows.Next() {
		var accountID, identityID string
		if err := rows.Scan(&accountID, &identityID); err != nil {
			return nil, err
		}
		name, err := c.identityName(ctx, c.db, identityID)
		if err != nil {
			return nil, err
		}
		if name != "" {
			names[name] = accountID
		}
	}
	return names, rows.Err()
}
