package auth

import (
	"context"
	"database/sql"
	"errors"
)

// NotifyPrefs are per-account or per-circle notification toggles.
type NotifyPrefs struct {
	Posts     bool `json:"posts"`
	Comments  bool `json:"comments"`
	Reactions bool `json:"reactions"`
	Mentions  bool `json:"mentions"`
}

// DefaultNotifyPrefs returns product defaults (mentions always on).
func DefaultNotifyPrefs() NotifyPrefs {
	return NotifyPrefs{Posts: true, Comments: true, Reactions: true, Mentions: true}
}

// AccountNotifyPrefs loads account-level defaults.
func (s *Service) AccountNotifyPrefs(ctx context.Context, accountID string) (NotifyPrefs, error) {
	prefs := DefaultNotifyPrefs()
	err := s.db.QueryRowContext(ctx, `
		SELECT posts, comments, reactions FROM account_notify_prefs WHERE account_id = ?
	`, accountID).Scan(&prefs.Posts, &prefs.Comments, &prefs.Reactions)
	if errors.Is(err, sql.ErrNoRows) {
		return prefs, nil
	}
	if err != nil {
		return NotifyPrefs{}, err
	}
	prefs.Mentions = true
	return prefs, nil
}

// SaveAccountNotifyPrefs stores account-level defaults.
func (s *Service) SaveAccountNotifyPrefs(ctx context.Context, accountID string, prefs NotifyPrefs) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO account_notify_prefs (account_id, posts, comments, reactions)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(account_id) DO UPDATE SET
			posts = excluded.posts,
			comments = excluded.comments,
			reactions = excluded.reactions
	`, accountID, boolInt(prefs.Posts), boolInt(prefs.Comments), boolInt(prefs.Reactions))
	return err
}

// CircleNotifyPrefs returns effective prefs for a circle (account defaults + optional override).
func (s *Service) CircleNotifyPrefs(ctx context.Context, accountID, circleID string) (NotifyPrefs, error) {
	base, err := s.AccountNotifyPrefs(ctx, accountID)
	if err != nil {
		return NotifyPrefs{}, err
	}
	var posts, comments, reactions sql.NullInt64
	err = s.db.QueryRowContext(ctx, `
		SELECT posts, comments, reactions FROM circle_notify_prefs
		WHERE account_id = ? AND circle_id = ?
	`, accountID, circleID).Scan(&posts, &comments, &reactions)
	if errors.Is(err, sql.ErrNoRows) {
		return base, nil
	}
	if err != nil {
		return NotifyPrefs{}, err
	}
	if posts.Valid {
		base.Posts = posts.Int64 == 1
	}
	if comments.Valid {
		base.Comments = comments.Int64 == 1
	}
	if reactions.Valid {
		base.Reactions = reactions.Int64 == 1
	}
	base.Mentions = true
	return base, nil
}

// SaveCircleNotifyPrefs stores a per-circle override; matching defaults removes the row.
func (s *Service) SaveCircleNotifyPrefs(ctx context.Context, accountID, circleID string, prefs, base NotifyPrefs) error {
	if prefs.Posts == base.Posts && prefs.Comments == base.Comments && prefs.Reactions == base.Reactions {
		_, err := s.db.ExecContext(ctx, `
			DELETE FROM circle_notify_prefs WHERE account_id = ? AND circle_id = ?
		`, accountID, circleID)
		return err
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO circle_notify_prefs (account_id, circle_id, posts, comments, reactions)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(account_id, circle_id) DO UPDATE SET
			posts = excluded.posts,
			comments = excluded.comments,
			reactions = excluded.reactions
	`, accountID, circleID, nullBool(prefs.Posts), nullBool(prefs.Comments), nullBool(prefs.Reactions))
	return err
}

// NotifyPrefAllows reports whether a signal type should be delivered.
func NotifyPrefAllows(prefs NotifyPrefs, signalType string) bool {
	switch signalType {
	case "post":
		return prefs.Posts
	case "comment":
		return prefs.Comments
	case "reaction":
		return prefs.Reactions
	case "mention":
		return true
	default:
		return true
	}
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func nullBool(v bool) any {
	if v {
		return 1
	}
	return 0
}
