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
	activity  int
}

// inviteActivityWindow — за какой срок считать активность для ряда 1.3.
const inviteActivityWindow = 90 * 24 * time.Hour

// InvitePeekForCircle returns member names for an invite link (no account ids).
// Порядок ряда вступления (wynd.html, «Вступление»): владелец, пригласивший,
// дальше самые активные — записи и комментарии в круге за 90 дней, при
// равенстве раньше вступивший. Новичок, который пишет, впереди молчаливого
// старожила: его пример лучше объясняет, как здесь зовутся.
func (c *Chronicle) InvitePeekForCircle(ctx context.Context, circleID, inviterAccountID string, now time.Time) (InvitePeek, error) {
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

	activity, err := c.inviteActivity(ctx, circleID, now.Add(-inviteActivityWindow))
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
			activity:  activity[identityID],
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
		if a.activity != b.activity {
			return a.activity > b.activity
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

// inviteActivity считает сказанное лицом в круге с since: неудалённые записи
// и комментарии. Реакции не в счёт — это не голос в журнале.
func (c *Chronicle) inviteActivity(ctx context.Context, circleID string, since time.Time) (map[string]int, error) {
	rows, err := c.db.QueryContext(ctx, `
		SELECT identity_id, COUNT(*) FROM (
			SELECT identity_id FROM posts
			WHERE circle_id = ? AND deleted = 0 AND created_at >= ?
			UNION ALL
			SELECT identity_id FROM comments
			WHERE circle_id = ? AND deleted = 0 AND created_at >= ?
		)
		GROUP BY identity_id
	`, circleID, formatTime(since), circleID, formatTime(since))
	if err != nil {
		return nil, fmt.Errorf("activity for invite peek: %w", err)
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var id string
		var n int
		if err := rows.Scan(&id, &n); err != nil {
			return nil, err
		}
		out[id] = n
	}
	return out, rows.Err()
}

// ServerInviteInviterName is the public name on a server invite (1.7): the
// live face of whoever created the link, or "" — then 1.7 shows only the
// address. The admin sentinel has no face, so admin links carry no name.
// It used to fall back to the owner of the oldest circle: an anonymous link
// holder saw a member's name, though that member invited no one.
func (c *Chronicle) ServerInviteInviterName(ctx context.Context, createdByAccountID string) (string, error) {
	if createdByAccountID == "" {
		return "", nil
	}
	name, err := c.accountFaceName(ctx, createdByAccountID)
	if err != nil && err != ErrNotFound {
		return "", err
	}
	return name, nil
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
