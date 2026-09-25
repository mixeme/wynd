package chronicle

import (
	"context"
	"fmt"
	"time"
)

// InviteCandidate is someone who can be invited from another shared circle.
type InviteCandidate struct {
	AccountID        string `json:"account_id"`
	Name             string `json:"name"`
	MembershipStatus string `json:"membership_status"`
	Invited          bool   `json:"invited"`
}

// InviteCandidateGroup lists candidates from one source circle.
type InviteCandidateGroup struct {
	ID      string            `json:"id"`
	Name    string            `json:"name"`
	Color   string            `json:"color"`
	Count   int               `json:"count"`
	Members []InviteCandidate `json:"members"`
}

// ListInviteCandidates returns people from the inviter's other circles who are not active in targetCircleID.
func (c *Chronicle) ListInviteCandidates(ctx context.Context, targetCircleID, inviterAccountID string, invited map[string]bool) ([]InviteCandidateGroup, error) {
	if err := c.RequireCanInvite(ctx, targetCircleID, inviterAccountID); err != nil {
		return nil, err
	}
	rows, err := c.db.QueryContext(ctx, `
		SELECT c.id, c.name, c.color, m.account_id, m.identity_id, m.status
		FROM memberships inv_src
		JOIN circles c ON c.id = inv_src.circle_id
		JOIN memberships m ON m.circle_id = inv_src.circle_id
		WHERE inv_src.account_id = ? AND inv_src.status = ?
		  AND inv_src.circle_id != ?
		  AND m.account_id != ? AND m.account_id != ''
		  AND m.status IN (?, ?, ?)
		  AND m.account_id NOT IN (
		    SELECT account_id FROM memberships WHERE circle_id = ? AND status = ?
		  )
		ORDER BY c.name COLLATE NOCASE, m.created_at
	`, inviterAccountID, StatusActive, targetCircleID, inviterAccountID,
		StatusActive, StatusLeftWithAccess, StatusGone,
		targetCircleID, StatusActive)
	if err != nil {
		return nil, fmt.Errorf("invite candidates: %w", err)
	}
	defer rows.Close()

	groups := []InviteCandidateGroup{}
	index := map[string]int{}
	for rows.Next() {
		var circleID, circleName, color, accountID, identityID, status string
		if err := rows.Scan(&circleID, &circleName, &color, &accountID, &identityID, &status); err != nil {
			return nil, err
		}
		name, err := c.identityName(ctx, c.db, identityID)
		if err != nil {
			return nil, err
		}
		candidate := InviteCandidate{
			AccountID:        accountID,
			Name:           name,
			MembershipStatus: status,
			Invited:        invited != nil && invited[accountID],
		}
		if i, ok := index[circleID]; ok {
			groups[i].Members = append(groups[i].Members, candidate)
			groups[i].Count = len(groups[i].Members)
		} else {
			index[circleID] = len(groups)
			groups = append(groups, InviteCandidateGroup{
				ID:      circleID,
				Name:    circleName,
				Color:   color,
				Count:   1,
				Members: []InviteCandidate{candidate},
			})
		}
	}
	return groups, rows.Err()
}

// CanInviteAccount reports whether target can be invited from inviter's other circles.
func (c *Chronicle) CanInviteAccount(ctx context.Context, targetCircleID, inviterAccountID, targetAccountID string) (bool, error) {
	if targetAccountID == "" || targetAccountID == inviterAccountID {
		return false, nil
	}
	active, err := c.membership(ctx, c.db, targetCircleID, targetAccountID)
	if err == nil && active.Status == StatusActive {
		return false, nil
	} else if err != nil && err != ErrNotFound {
		return false, err
	}
	var n int
	err = c.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM memberships inv_src
		JOIN memberships m ON m.circle_id = inv_src.circle_id
		WHERE inv_src.account_id = ? AND inv_src.status = ?
		  AND inv_src.circle_id != ?
		  AND m.account_id = ? AND m.status IN (?, ?, ?)
	`, inviterAccountID, StatusActive, targetCircleID, targetAccountID,
		StatusActive, StatusLeftWithAccess, StatusGone).Scan(&n)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// RecordMemberInvited appends a service event without journal text (push uses events pref).
func (c *Chronicle) RecordMemberInvited(ctx context.Context, circleID, inviterAccountID string, now time.Time) error {
	mem, err := c.membership(ctx, c.db, circleID, inviterAccountID)
	if err != nil {
		return err
	}
	name, err := c.identityName(ctx, c.db, mem.IdentityID)
	if err != nil {
		return err
	}
	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := c.appendEvent(ctx, tx, appendEventInput{
		circleID:        circleID,
		eventType:       "member.invited",
		isService:       true,
		actorIdentityID: mem.IdentityID,
		actorName:       name,
		summary:         "",
		now:             now.UTC(),
	}); err != nil {
		return err
	}
	return tx.Commit()
}
