package auth

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// NotifyPrefs are per-account or per-circle notification toggles.
type NotifyPrefs struct {
	Posts        bool    `json:"posts"`
	CommentsMine bool    `json:"comments_mine"`
	CommentsAll  bool    `json:"comments_all"`
	Reactions    bool    `json:"reactions"`
	Mentions     bool    `json:"mentions"`
	Events       bool    `json:"events"`
	MuteUntil    *string `json:"mute_until,omitempty"`
}

// DefaultNotifyPrefs returns product defaults (mentions always on, reactions off).
func DefaultNotifyPrefs() NotifyPrefs {
	return NotifyPrefs{
		Posts:        true,
		CommentsMine: true,
		CommentsAll:  false,
		Reactions:    false,
		Mentions:     true,
		Events:       false,
	}
}

func scanNotifyPrefs(posts, commentsMine, commentsAll, reactions, events sql.NullInt64, muteUntil sql.NullString, fallbackComments sql.NullInt64) NotifyPrefs {
	prefs := DefaultNotifyPrefs()
	if posts.Valid {
		prefs.Posts = posts.Int64 == 1
	}
	if commentsMine.Valid {
		prefs.CommentsMine = commentsMine.Int64 == 1
	} else if fallbackComments.Valid {
		prefs.CommentsMine = fallbackComments.Int64 == 1
	}
	if commentsAll.Valid {
		prefs.CommentsAll = commentsAll.Int64 == 1
	}
	if reactions.Valid {
		prefs.Reactions = reactions.Int64 == 1
	}
	if events.Valid {
		prefs.Events = events.Int64 == 1
	}
	if muteUntil.Valid && muteUntil.String != "" {
		v := muteUntil.String
		prefs.MuteUntil = &v
	}
	prefs.Mentions = true
	return prefs
}

// AccountNotifyPrefs loads account-level defaults.
func (s *Service) AccountNotifyPrefs(ctx context.Context, accountID string) (NotifyPrefs, error) {
	var posts, comments, commentsMine, commentsAll, reactions, events sql.NullInt64
	var muteUntil sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT posts, comments, comments_mine, comments_all, reactions, events, mute_until
		FROM account_notify_prefs WHERE account_id = ?
	`, accountID).Scan(&posts, &comments, &commentsMine, &commentsAll, &reactions, &events, &muteUntil)
	if errors.Is(err, sql.ErrNoRows) {
		return DefaultNotifyPrefs(), nil
	}
	if err != nil {
		return NotifyPrefs{}, err
	}
	return scanNotifyPrefs(posts, commentsMine, commentsAll, reactions, events, muteUntil, comments), nil
}

// SaveAccountNotifyPrefs stores account-level defaults.
func (s *Service) SaveAccountNotifyPrefs(ctx context.Context, accountID string, prefs NotifyPrefs) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO account_notify_prefs (
			account_id, posts, comments, comments_mine, comments_all, reactions, events, mute_until
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(account_id) DO UPDATE SET
			posts = excluded.posts,
			comments = excluded.comments_mine,
			comments_mine = excluded.comments_mine,
			comments_all = excluded.comments_all,
			reactions = excluded.reactions,
			events = excluded.events,
			mute_until = excluded.mute_until
	`, accountID,
		boolInt(prefs.Posts),
		boolInt(prefs.CommentsMine),
		boolInt(prefs.CommentsMine),
		boolInt(prefs.CommentsAll),
		boolInt(prefs.Reactions),
		boolInt(prefs.Events),
		nullString(prefs.MuteUntil),
	)
	return err
}

// CircleNotifyPrefs returns effective prefs for a circle (account defaults + optional override).
func (s *Service) CircleNotifyPrefs(ctx context.Context, accountID, circleID string) (NotifyPrefs, error) {
	base, err := s.AccountNotifyPrefs(ctx, accountID)
	if err != nil {
		return NotifyPrefs{}, err
	}
	var posts, comments, commentsMine, commentsAll, reactions, events sql.NullInt64
	var muteUntil sql.NullString
	err = s.db.QueryRowContext(ctx, `
		SELECT posts, comments, comments_mine, comments_all, reactions, events, mute_until
		FROM circle_notify_prefs
		WHERE account_id = ? AND circle_id = ?
	`, accountID, circleID).Scan(&posts, &comments, &commentsMine, &commentsAll, &reactions, &events, &muteUntil)
	if errors.Is(err, sql.ErrNoRows) {
		return base, nil
	}
	if err != nil {
		return NotifyPrefs{}, err
	}
	out := base
	if posts.Valid {
		out.Posts = posts.Int64 == 1
	}
	if commentsMine.Valid {
		out.CommentsMine = commentsMine.Int64 == 1
	} else if comments.Valid {
		out.CommentsMine = comments.Int64 == 1
	}
	if commentsAll.Valid {
		out.CommentsAll = commentsAll.Int64 == 1
	}
	if reactions.Valid {
		out.Reactions = reactions.Int64 == 1
	}
	if events.Valid {
		out.Events = events.Int64 == 1
	}
	if muteUntil.Valid {
		if muteUntil.String == "" {
			out.MuteUntil = nil
		} else {
			v := muteUntil.String
			out.MuteUntil = &v
		}
	}
	out.Mentions = true
	return out, nil
}

func notifyPrefsEqual(a, b NotifyPrefs) bool {
	return a.Posts == b.Posts &&
		a.CommentsMine == b.CommentsMine &&
		a.CommentsAll == b.CommentsAll &&
		a.Reactions == b.Reactions &&
		a.Events == b.Events &&
		muteUntilEqual(a.MuteUntil, b.MuteUntil)
}

func muteUntilEqual(a, b *string) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

// SaveCircleNotifyPrefs stores a per-circle override; matching defaults removes the row.
func (s *Service) SaveCircleNotifyPrefs(ctx context.Context, accountID, circleID string, prefs, base NotifyPrefs) error {
	if notifyPrefsEqual(prefs, base) {
		_, err := s.db.ExecContext(ctx, `
			DELETE FROM circle_notify_prefs WHERE account_id = ? AND circle_id = ?
		`, accountID, circleID)
		return err
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO circle_notify_prefs (
			account_id, circle_id, posts, comments, comments_mine, comments_all, reactions, events, mute_until
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(account_id, circle_id) DO UPDATE SET
			posts = excluded.posts,
			comments = excluded.comments_mine,
			comments_mine = excluded.comments_mine,
			comments_all = excluded.comments_all,
			reactions = excluded.reactions,
			events = excluded.events,
			mute_until = excluded.mute_until
	`, accountID, circleID,
		nullBool(prefs.Posts),
		nullBool(prefs.CommentsMine),
		nullBool(prefs.CommentsMine),
		nullBool(prefs.CommentsAll),
		nullBool(prefs.Reactions),
		nullBool(prefs.Events),
		nullString(prefs.MuteUntil),
	)
	return err
}

// NotifyPrefAllows reports whether a signal type should be delivered.
func NotifyPrefAllows(prefs NotifyPrefs, signalType string, now time.Time) bool {
	// Упоминание пробивает тишину. Личное «позвать» не смотрит на «события»:
	// они по умолчанию выключены, и приглашение вышедшего пропадало молча.
	// Тишина на срок приглашение всё же держит.
	if signalType == "mention" {
		return true
	}
	if NotifyMuted(prefs, now) {
		return false
	}
	if signalType == "invite" {
		return true
	}
	switch signalType {
	case "post":
		return prefs.Posts
	case "comment":
		return false
	case "reaction":
		return prefs.Reactions
	case "event":
		return prefs.Events
	default:
		return true
	}
}

// NotifyCommentAllows reports whether a comment notification should be delivered.
func NotifyCommentAllows(prefs NotifyPrefs, isPostAuthor bool, now time.Time) bool {
	if NotifyMuted(prefs, now) {
		return false
	}
	if isPostAuthor && prefs.CommentsMine {
		return true
	}
	return prefs.CommentsAll
}

// NotifyMuted reports whether notifications are muted until a future instant.
func NotifyMuted(prefs NotifyPrefs, now time.Time) bool {
	if prefs.MuteUntil == nil || *prefs.MuteUntil == "" {
		return false
	}
	t, err := time.Parse(time.RFC3339, *prefs.MuteUntil)
	if err != nil {
		return false
	}
	return now.Before(t)
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

func nullString(v *string) any {
	if v == nil || *v == "" {
		return nil
	}
	return *v
}
