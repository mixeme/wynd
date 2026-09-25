package chronicle

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"time"
)

// InvitePeekMember is a circle participant visible on invite screens (names only).
type InvitePeekMember struct {
	Name      string `json:"name"`
	IsOwner   bool   `json:"is_owner"`
	IsInviter bool   `json:"is_inviter"`
}

// InvitePeek is public invite metadata for pre-auth screens.
type InvitePeek struct {
	CircleName  string             `json:"circle_name"`
	Color       string             `json:"color"`
	MemberCount int                `json:"member_count"`
	Members     []InvitePeekMember `json:"members"`
}

type inviteMemberRow struct {
	accountID string
	name      string
	isOwner   bool
	joinedAt  time.Time
}

// InvitePeekForCircle returns member names for an invite link (no account ids).
func (c *Chronicle) InvitePeekForCircle(ctx context.Context, circleID, inviterAccountID string) (InvitePeek, error) {
	var name, color string
	err := c.db.QueryRowContext(ctx, `
		SELECT name, color FROM circles WHERE id = ?
	`, circleID).Scan(&name, &color)
	if err == sql.ErrNoRows {
		return InvitePeek{}, ErrNotFound
	}
	if err != nil {
		return InvitePeek{}, fmt.Errorf("circle for invite peek: %w", err)
	}

	owner, err := c.circleOwner(ctx, c.db, circleID)
	if err != nil {
		return InvitePeek{}, err
	}

	rows, err := c.db.QueryContext(ctx, `
		SELECT m.account_id, m.identity_id, m.created_at
		FROM memberships m
		WHERE m.circle_id = ? AND m.status = ?
		ORDER BY m.created_at
	`, circleID, StatusActive)
	if err != nil {
		return InvitePeek{}, fmt.Errorf("members for invite peek: %w", err)
	}
	defer rows.Close()

	var raw []inviteMemberRow
	for rows.Next() {
		var accountID, identityID, created string
		if err := rows.Scan(&accountID, &identityID, &created); err != nil {
			return InvitePeek{}, err
		}
		displayName, err := c.identityName(ctx, c.db, identityID)
		if err != nil {
			return InvitePeek{}, err
		}
		joinedAt, err := parseTime(created)
		if err != nil {
			return InvitePeek{}, err
		}
		raw = append(raw, inviteMemberRow{
			accountID: accountID,
			name:      displayName,
			isOwner:   accountID == owner,
			joinedAt:  joinedAt,
		})
	}
	if err := rows.Err(); err != nil {
		return InvitePeek{}, err
	}

	sort.SliceStable(raw, func(i, j int) bool {
		a, b := raw[i], raw[j]
		if a.isOwner != b.isOwner {
			return a.isOwner
		}
		if inviterAccountID != "" {
			if a.accountID == inviterAccountID {
				return true
			}
			if b.accountID == inviterAccountID {
				return false
			}
		}
		return a.joinedAt.Before(b.joinedAt)
	})

	members := make([]InvitePeekMember, len(raw))
	for i, row := range raw {
		members[i] = InvitePeekMember{
			Name:      row.name,
			IsOwner:   row.isOwner,
			IsInviter: inviterAccountID != "" && row.accountID == inviterAccountID,
		}
	}

	return InvitePeek{
		CircleName:  name,
		Color:       color,
		MemberCount: len(members),
		Members:     members,
	}, nil
}

// ServerInviteInviterName is the public name on a server invite (1.7).
// Prefer a live identity of who created the link; if that account has no face
// (admin sentinel), the owner of the oldest circle on the instance.
func (c *Chronicle) ServerInviteInviterName(ctx context.Context, createdByAccountID string) (string, error) {
	if createdByAccountID != "" {
		name, err := c.accountFaceName(ctx, createdByAccountID)
		if err != nil && err != ErrNotFound {
			return "", err
		}
		if name != "" {
			return name, nil
		}
	}
	return c.oldestCircleOwnerName(ctx)
}

func (c *Chronicle) accountFaceName(ctx context.Context, accountID string) (string, error) {
	var name string
	err := c.db.QueryRowContext(ctx, `
		SELECT n.name
		FROM memberships m
		JOIN identity_names n ON n.identity_id = m.identity_id AND n.erased_at IS NULL
		WHERE m.account_id = ? AND m.status = ?
		ORDER BY m.created_at ASC, n.effective_at DESC
		LIMIT 1
	`, accountID, StatusActive).Scan(&name)
	if err == sql.ErrNoRows {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("account face for invite peek: %w", err)
	}
	return name, nil
}

func (c *Chronicle) oldestCircleOwnerName(ctx context.Context) (string, error) {
	var name string
	err := c.db.QueryRowContext(ctx, `
		SELECT n.name
		FROM circles c
		JOIN memberships m ON m.circle_id = c.id AND m.account_id = c.owner_account_id AND m.status = ?
		JOIN identity_names n ON n.identity_id = m.identity_id AND n.erased_at IS NULL
		ORDER BY c.created_at ASC, n.effective_at DESC
		LIMIT 1
	`, StatusActive).Scan(&name)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("oldest circle owner for invite peek: %w", err)
	}
	return name, nil
}
