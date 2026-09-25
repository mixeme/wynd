package chronicle

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

func (c *Chronicle) ensureDay(ctx context.Context, tx *sql.Tx, circleID, entryDate string) error {
	_, err := tx.ExecContext(ctx, `
		INSERT OR IGNORE INTO days (circle_id, entry_date) VALUES (?, ?)
	`, circleID, entryDate)
	return err
}

func (c *Chronicle) maybeCollapseDay(ctx context.Context, tx *sql.Tx, circleID, entryDate string) error {
	var n int
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM posts WHERE circle_id = ? AND entry_date = ? AND deleted = 0
	`, circleID, entryDate).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM days WHERE circle_id = ? AND entry_date = ?`, circleID, entryDate); err != nil {
		return err
	}
	// User content of that day: gone from the journal, not a "deleted" line.
	_, err := tx.ExecContext(ctx, `
		DELETE FROM events
		WHERE circle_id = ?
		  AND event_type IN ('day.titled', 'day.cover_set')
		  AND json_extract(payload, '$.entry_date') = ?
	`, circleID, entryDate)
	return err
}

func (c *Chronicle) reconcileDayAfterEntryDateChange(ctx context.Context, tx *sql.Tx, circleID, oldDate, newDate string) error {
	if err := c.maybeCollapseDay(ctx, tx, circleID, oldDate); err != nil {
		return err
	}
	if err := c.reconcileDayCoverAfterPostGone(ctx, tx, circleID, oldDate); err != nil {
		return err
	}
	return c.ensureDay(ctx, tx, circleID, newDate)
}

func (c *Chronicle) reconcileDayCoverAfterPostGone(ctx context.Context, tx *sql.Tx, circleID, entryDate string) error {
	exists, err := c.dayExistsTx(ctx, tx, circleID, entryDate)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	rows, err := tx.QueryContext(ctx, `
		SELECT dc.event_seq
		FROM day_covers dc
		JOIN posts p ON p.id = dc.post_id
		WHERE dc.circle_id = ? AND dc.entry_date = ? AND p.deleted = 1
	`, circleID, entryDate)
	if err != nil {
		return err
	}
	defer rows.Close()
	var seqs []int64
	for rows.Next() {
		var seq int64
		if err := rows.Scan(&seq); err != nil {
			return err
		}
		seqs = append(seqs, seq)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if err := c.removeSaidHistory(ctx, tx, seqs); err != nil {
		return err
	}
	return c.reprojectDayCover(ctx, tx, circleID, entryDate)
}

// reconcileDayCoverAfterBlobRemovedFromPost drops day cover history when that blob is no longer on the post.
func (c *Chronicle) reconcileDayCoverAfterBlobRemovedFromPost(ctx context.Context, tx *sql.Tx, circleID, entryDate, postID, blobID string) error {
	rows, err := tx.QueryContext(ctx, `
		SELECT dc.event_seq
		FROM day_covers dc
		JOIN posts p ON p.id = dc.post_id AND p.deleted = 0
		WHERE dc.circle_id = ? AND dc.entry_date = ? AND dc.post_id = ? AND dc.blob_id = ?
	`, circleID, entryDate, postID, blobID)
	if err != nil {
		return err
	}
	defer rows.Close()
	var seqs []int64
	for rows.Next() {
		var seq int64
		if err := rows.Scan(&seq); err != nil {
			return err
		}
		seqs = append(seqs, seq)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if len(seqs) == 0 {
		return nil
	}
	if err := c.removeSaidHistory(ctx, tx, seqs); err != nil {
		return err
	}
	return c.reprojectDayCover(ctx, tx, circleID, entryDate)
}

func (c *Chronicle) dayExistsTx(ctx context.Context, tx *sql.Tx, circleID, entryDate string) (bool, error) {
	var n int
	err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM days WHERE circle_id = ? AND entry_date = ?
	`, circleID, entryDate).Scan(&n)
	return n > 0, err
}

func (c *Chronicle) removeSaidHistory(ctx context.Context, tx *sql.Tx, seqs []int64) error {
	for _, seq := range seqs {
		if _, err := tx.ExecContext(ctx, `DELETE FROM day_titles WHERE event_seq = ?`, seq); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM day_covers WHERE event_seq = ?`, seq); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM events WHERE seq = ?`, seq); err != nil {
			return err
		}
	}
	return nil
}

func (c *Chronicle) reprojectDayTitle(ctx context.Context, tx *sql.Tx, circleID, entryDate string) error {
	var title sql.NullString
	var seq sql.NullInt64
	err := tx.QueryRowContext(ctx, `
		SELECT title, event_seq FROM day_titles
		WHERE circle_id = ? AND entry_date = ?
		ORDER BY created_at DESC, event_seq DESC LIMIT 1
	`, circleID, entryDate).Scan(&title, &seq)
	if err == sql.ErrNoRows {
		_, err = tx.ExecContext(ctx, `
			UPDATE days SET title = NULL, title_event_seq = NULL
			WHERE circle_id = ? AND entry_date = ?
		`, circleID, entryDate)
		return err
	}
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `
		UPDATE days SET title = ?, title_event_seq = ? WHERE circle_id = ? AND entry_date = ?
	`, title, seq, circleID, entryDate)
	return err
}

func (c *Chronicle) reprojectDayCover(ctx context.Context, tx *sql.Tx, circleID, entryDate string) error {
	var postID, blobID sql.NullString
	var seq sql.NullInt64
	err := tx.QueryRowContext(ctx, `
		SELECT dc.post_id, dc.blob_id, dc.event_seq
		FROM day_covers dc
		JOIN posts p ON p.id = dc.post_id AND p.deleted = 0
		WHERE dc.circle_id = ? AND dc.entry_date = ?
		ORDER BY dc.created_at DESC, dc.event_seq DESC LIMIT 1
	`, circleID, entryDate).Scan(&postID, &blobID, &seq)
	if err == sql.ErrNoRows {
		_, err = tx.ExecContext(ctx, `
			UPDATE days SET cover_post_id = NULL, cover_blob_id = NULL, cover_event_seq = NULL
			WHERE circle_id = ? AND entry_date = ?
		`, circleID, entryDate)
		return err
	}
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `
		UPDATE days SET cover_post_id = ?, cover_blob_id = ?, cover_event_seq = ?
		WHERE circle_id = ? AND entry_date = ?
	`, postID, blobID, seq, circleID, entryDate)
	return err
}

type daySaidRow struct {
	eventSeq   int64
	createdAt  time.Time
	editWindow EditWindow
}

func (c *Chronicle) currentDayTitle(ctx context.Context, q querier, circleID, entryDate string) (daySaidRow, error) {
	var row daySaidRow
	var created string
	var ew sql.NullInt64
	err := q.QueryRowContext(ctx, `
		SELECT event_seq, created_at, edit_window_sec
		FROM day_titles
		WHERE circle_id = ? AND entry_date = ?
		ORDER BY created_at DESC, event_seq DESC LIMIT 1
	`, circleID, entryDate).Scan(&row.eventSeq, &created, &ew)
	if err == sql.ErrNoRows {
		return daySaidRow{}, ErrNotFound
	}
	if err != nil {
		return daySaidRow{}, err
	}
	row.createdAt, _ = parseTime(created)
	row.editWindow, _ = editWindowFromSQL(ew)
	return row, nil
}

func (c *Chronicle) currentLiveDayCover(ctx context.Context, q querier, circleID, entryDate string) (daySaidRow, error) {
	var row daySaidRow
	var created string
	var ew sql.NullInt64
	err := q.QueryRowContext(ctx, `
		SELECT dc.event_seq, dc.created_at, dc.edit_window_sec
		FROM day_covers dc
		JOIN posts p ON p.id = dc.post_id AND p.deleted = 0
		WHERE dc.circle_id = ? AND dc.entry_date = ?
		ORDER BY dc.created_at DESC, dc.event_seq DESC LIMIT 1
	`, circleID, entryDate).Scan(&row.eventSeq, &created, &ew)
	if err == sql.ErrNoRows {
		return daySaidRow{}, ErrNotFound
	}
	if err != nil {
		return daySaidRow{}, err
	}
	row.createdAt, _ = parseTime(created)
	row.editWindow, _ = editWindowFromSQL(ew)
	return row, nil
}

func (c *Chronicle) dayTitleEditableUntil(ctx context.Context, circleID, entryDate string) (*time.Time, error) {
	var until sql.NullString
	err := c.db.QueryRowContext(ctx, `
		SELECT editable_until FROM day_titles
		WHERE circle_id = ? AND entry_date = ?
		ORDER BY created_at DESC, event_seq DESC LIMIT 1
	`, circleID, entryDate).Scan(&until)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return parseEditableUntil(until)
}

func (c *Chronicle) dayCoverEditableUntil(ctx context.Context, circleID, entryDate string) (*time.Time, error) {
	var until sql.NullString
	err := c.db.QueryRowContext(ctx, `
		SELECT dc.editable_until
		FROM day_covers dc
		JOIN posts p ON p.id = dc.post_id AND p.deleted = 0
		WHERE dc.circle_id = ? AND dc.entry_date = ?
		ORDER BY dc.created_at DESC, dc.event_seq DESC LIMIT 1
	`, circleID, entryDate).Scan(&until)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return parseEditableUntil(until)
}

func (c *Chronicle) hasPostForDay(ctx context.Context, q querier, circleID, accountID, entryDate string) (bool, error) {
	mem, err := c.membership(ctx, q, circleID, accountID)
	if err != nil {
		return false, err
	}
	var n int
	err = q.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM posts
		WHERE circle_id = ? AND identity_id = ? AND entry_date = ? AND deleted = 0
	`, circleID, mem.IdentityID, entryDate).Scan(&n)
	return n > 0, err
}

// SetDayTitle sets day title (last-write-wins projection).
func (c *Chronicle) SetDayTitle(ctx context.Context, in DayTitleInput) error {
	if in.Title == "" {
		return ErrInvalid
	}
	if err := checkByteLen(in.Title, MaxTextBytes); err != nil {
		return err
	}
	now := utcOrNow(in.Now)
	ok, err := c.hasPostForDay(ctx, c.db, in.CircleID, in.AccountID, in.EntryDate)
	if err != nil {
		return err
	}
	if !ok {
		return ErrForbidden
	}
	mem, err := c.requireWriter(ctx, in.CircleID, in.AccountID, now)
	if err != nil {
		return err
	}
	window, err := c.circleEditWindow(ctx, c.db, in.CircleID)
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

	if err := c.ensureDay(ctx, tx, in.CircleID, in.EntryDate); err != nil {
		return err
	}

	ev, err := c.appendEvent(ctx, tx, appendEventInput{
		circleID: in.CircleID, eventType: "day.titled", isService: false,
		actorIdentityID: mem.IdentityID, actorName: name,
		payload: map[string]any{"entry_date": in.EntryDate, "title": in.Title},
		summary: summaryDayTitled(name, in.Title), now: now,
	})
	if err != nil {
		return err
	}
	until := window.EditableUntil(now)
	_, err = tx.ExecContext(ctx, `
		INSERT INTO day_titles (
			circle_id, entry_date, event_seq, identity_id, title, created_at,
			edit_window_sec, editable_until
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, in.CircleID, in.EntryDate, ev.Seq, mem.IdentityID, in.Title, formatTime(now),
		editWindowToSQL(window), editableUntilToSQL(until))
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `
		UPDATE days SET title = ?, title_event_seq = ? WHERE circle_id = ? AND entry_date = ?
	`, in.Title, ev.Seq, in.CircleID, in.EntryDate)
	if err != nil {
		return err
	}
	return tx.Commit()
}

// SetDayCover sets day cover (last-write-wins projection).
func (c *Chronicle) SetDayCover(ctx context.Context, in DayCoverInput) error {
	if in.PostID == "" || in.BlobID == "" {
		return ErrInvalid
	}
	now := utcOrNow(in.Now)
	ok, err := c.hasPostForDay(ctx, c.db, in.CircleID, in.AccountID, in.EntryDate)
	if err != nil {
		return err
	}
	if !ok {
		return ErrForbidden
	}
	post, err := c.loadPost(ctx, c.db, in.PostID)
	if err != nil {
		return err
	}
	if post.CircleID != in.CircleID || post.EntryDate != in.EntryDate || post.Deleted {
		return ErrInvalid
	}
	okBlob, err := c.BlobOnPost(ctx, in.PostID, in.BlobID)
	if err != nil {
		return err
	}
	if !okBlob {
		return ErrInvalid
	}
	mem, err := c.requireWriter(ctx, in.CircleID, in.AccountID, now)
	if err != nil {
		return err
	}
	window, err := c.circleEditWindow(ctx, c.db, in.CircleID)
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

	if err := c.ensureDay(ctx, tx, in.CircleID, in.EntryDate); err != nil {
		return err
	}

	ev, err := c.appendEvent(ctx, tx, appendEventInput{
		circleID: in.CircleID, eventType: "day.cover_set", isService: false,
		actorIdentityID: mem.IdentityID, actorName: name,
		payload: map[string]any{
			"entry_date": in.EntryDate, "post_id": in.PostID, "blob_id": in.BlobID,
		},
		summary: summaryDayCoverSet(name), now: now,
	})
	if err != nil {
		return err
	}
	until := window.EditableUntil(now)
	_, err = tx.ExecContext(ctx, `
		INSERT INTO day_covers (
			circle_id, entry_date, event_seq, identity_id, post_id, blob_id, created_at,
			edit_window_sec, editable_until
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, in.CircleID, in.EntryDate, ev.Seq, mem.IdentityID, in.PostID, in.BlobID, formatTime(now),
		editWindowToSQL(window), editableUntilToSQL(until))
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `
		UPDATE days SET cover_post_id = ?, cover_blob_id = ?, cover_event_seq = ?
		WHERE circle_id = ? AND entry_date = ?
	`, in.PostID, in.BlobID, ev.Seq, in.CircleID, in.EntryDate)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (c *Chronicle) canClearDaySaid(ctx context.Context, circleID, accountID, entryDate string, now time.Time, current daySaidRow, errCurrent error) error {
	if errCurrent != nil {
		if errCurrent == ErrNotFound {
			return ErrInvalid
		}
		return errCurrent
	}
	ok, err := c.hasPostForDay(ctx, c.db, circleID, accountID, entryDate)
	if err != nil {
		return err
	}
	if !ok {
		return ErrForbidden
	}
	if !current.editWindow.CanEdit(current.createdAt, now) {
		return ErrForbidden
	}
	return nil
}

// ClearDayTitle removes the current day title from the journal and falls back to the previous one.
func (c *Chronicle) ClearDayTitle(ctx context.Context, circleID, accountID, entryDate string, now time.Time) error {
	now = utcOrNow(now)
	current, err := c.currentDayTitle(ctx, c.db, circleID, entryDate)
	if err := c.canClearDaySaid(ctx, circleID, accountID, entryDate, now, current, err); err != nil {
		return err
	}

	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if err := c.removeSaidHistory(ctx, tx, []int64{current.eventSeq}); err != nil {
		return err
	}
	if err := c.reprojectDayTitle(ctx, tx, circleID, entryDate); err != nil {
		return err
	}
	return tx.Commit()
}

// ClearDayCover removes the current day cover from the journal and falls back to the previous live one.
func (c *Chronicle) ClearDayCover(ctx context.Context, circleID, accountID, entryDate string, now time.Time) error {
	now = utcOrNow(now)
	current, err := c.currentLiveDayCover(ctx, c.db, circleID, entryDate)
	if err := c.canClearDaySaid(ctx, circleID, accountID, entryDate, now, current, err); err != nil {
		return err
	}

	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if err := c.removeSaidHistory(ctx, tx, []int64{current.eventSeq}); err != nil {
		return err
	}
	if err := c.reprojectDayCover(ctx, tx, circleID, entryDate); err != nil {
		return err
	}
	return tx.Commit()
}

// GetDay returns the day projection.
func (c *Chronicle) GetDay(ctx context.Context, circleID, entryDate string) (Day, error) {
	var d Day
	var title, coverPost, coverBlob sql.NullString
	err := c.db.QueryRowContext(ctx, `
		SELECT circle_id, entry_date, title, cover_post_id, cover_blob_id
		FROM days WHERE circle_id = ? AND entry_date = ?
	`, circleID, entryDate).Scan(&d.CircleID, &d.EntryDate, &title, &coverPost, &coverBlob)
	if err == sql.ErrNoRows {
		return Day{}, ErrNotFound
	}
	if err != nil {
		return Day{}, err
	}
	if title.Valid {
		d.Title = title.String
	}
	if coverPost.Valid {
		d.CoverPostID = coverPost.String
	}
	if coverBlob.Valid {
		d.CoverBlobID = coverBlob.String
	}
	return d, nil
}

// DayExists reports whether a day projection exists.
func (c *Chronicle) DayExists(ctx context.Context, circleID, entryDate string) (bool, error) {
	var n int
	err := c.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM days WHERE circle_id = ? AND entry_date = ?
	`, circleID, entryDate).Scan(&n)
	return n > 0, err
}

// CountPostsForDay counts non-deleted posts for entry date.
func (c *Chronicle) CountPostsForDay(ctx context.Context, circleID, entryDate string) (int, error) {
	var n int
	err := c.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM posts WHERE circle_id = ? AND entry_date = ? AND deleted = 0
	`, circleID, entryDate).Scan(&n)
	return n, err
}

// CountCirclePosts counts non-deleted posts in a circle.
func (c *Chronicle) CountCirclePosts(ctx context.Context, circleID string) (int, error) {
	var n int
	err := c.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM posts WHERE circle_id = ? AND deleted = 0
	`, circleID).Scan(&n)
	return n, err
}

// FeedOrder returns post IDs ordered by created_at (not entry_date).
func (c *Chronicle) FeedOrder(ctx context.Context, circleID string) ([]string, error) {
	rows, err := c.db.QueryContext(ctx, `
		SELECT id FROM posts WHERE circle_id = ? AND deleted = 0 ORDER BY created_at
	`, circleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// PostBody returns stored body text (empty if deleted).
func (c *Chronicle) PostBody(ctx context.Context, postID string) (string, error) {
	var body sql.NullString
	var deleted int
	err := c.db.QueryRowContext(ctx, `SELECT body, deleted FROM posts WHERE id = ?`, postID).Scan(&body, &deleted)
	if err == sql.ErrNoRows {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	if deleted == 1 {
		return "", nil
	}
	if body.Valid {
		return body.String, nil
	}
	return "", nil
}

// CommentBodies returns comment bodies for a post.
func (c *Chronicle) CommentBodies(ctx context.Context, postID string) ([]string, error) {
	rows, err := c.db.QueryContext(ctx, `
		SELECT body FROM comments WHERE post_id = ?
	`, postID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var body sql.NullString
		if err := rows.Scan(&body); err != nil {
			return nil, err
		}
		if body.Valid {
			out = append(out, body.String)
		}
	}
	return out, rows.Err()
}

// EventCount returns number of events (for GDPR test).
func (c *Chronicle) EventCount(ctx context.Context, circleID string) (int, error) {
	var n int
	err := c.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM events WHERE circle_id = ?`, circleID).Scan(&n)
	return n, err
}

// ResolveIdentityName returns current name or empty if erased.
func (c *Chronicle) ResolveIdentityName(ctx context.Context, identityID string) (string, error) {
	return c.identityName(ctx, c.db, identityID)
}

// ErasePersonalData removes identity names without touching events.
func (c *Chronicle) ErasePersonalData(ctx context.Context, identityID string, now time.Time) error {
	_, err := c.db.ExecContext(ctx, `
		UPDATE identity_names SET name = '', erased_at = ? WHERE identity_id = ? AND erased_at IS NULL
	`, formatTime(now), identityID)
	return err
}

// LoadPostEditWindow returns the snapshot edit window stored on a post.
func (c *Chronicle) LoadPostEditWindow(ctx context.Context, postID string) (EditWindow, time.Time, error) {
	post, err := c.loadPost(ctx, c.db, postID)
	if err != nil {
		return EditWindow{}, time.Time{}, err
	}
	return post.EditWindow, post.CreatedAt, nil
}

// LoadCommentEditWindow returns snapshot on a comment.
func (c *Chronicle) LoadCommentEditWindow(ctx context.Context, commentID string) (EditWindow, time.Time, error) {
	var ew sql.NullInt64
	var created string
	err := c.db.QueryRowContext(ctx, `
		SELECT edit_window_sec, created_at FROM comments WHERE id = ?
	`, commentID).Scan(&ew, &created)
	if err == sql.ErrNoRows {
		return EditWindow{}, time.Time{}, ErrNotFound
	}
	if err != nil {
		return EditWindow{}, time.Time{}, err
	}
	window, err := editWindowFromSQL(ew)
	if err != nil {
		return EditWindow{}, time.Time{}, err
	}
	t, _ := parseTime(created)
	return window, t, nil
}

// FindCommentID returns first comment id for post/account (test helper).
func (c *Chronicle) FindCommentID(ctx context.Context, postID, accountID string) (string, error) {
	var id string
	err := c.db.QueryRowContext(ctx, `
		SELECT c.id FROM comments c
		JOIN memberships m ON m.identity_id = c.identity_id
		WHERE c.post_id = ? AND m.account_id = ? AND c.deleted = 0
		LIMIT 1
	`, postID, accountID).Scan(&id)
	if err == sql.ErrNoRows {
		return "", ErrNotFound
	}
	return id, err
}

// FindReactionID returns the live reaction id for post/account (test helper).
func (c *Chronicle) FindReactionID(ctx context.Context, postID, accountID string) (string, error) {
	var id string
	err := c.db.QueryRowContext(ctx, `
		SELECT r.id FROM reactions r
		JOIN memberships m ON m.identity_id = r.identity_id
		WHERE r.post_id = ? AND m.account_id = ? AND r.deleted = 0
		LIMIT 1
	`, postID, accountID).Scan(&id)
	if err == sql.ErrNoRows {
		return "", ErrNotFound
	}
	return id, err
}

// CommentBody returns stored comment text (empty if deleted).
func (c *Chronicle) CommentBody(ctx context.Context, commentID string) (string, error) {
	cm, err := c.loadComment(ctx, c.db, commentID)
	if err != nil {
		return "", err
	}
	if cm.Deleted {
		return "", nil
	}
	return cm.Body, nil
}

// ReactionEmoji returns stored emoji (empty if deleted).
func (c *Chronicle) ReactionEmoji(ctx context.Context, reactionID string) (string, error) {
	r, err := c.loadReaction(ctx, c.db, reactionID)
	if err != nil {
		return "", err
	}
	if r.Deleted {
		return "", nil
	}
	return r.Emoji, nil
}

// ServiceEventSummary fetches human-readable summary for an event.
func (c *Chronicle) ServiceEventSummary(ctx context.Context, eventSeq int64) (string, error) {
	var summary string
	err := c.db.QueryRowContext(ctx, `SELECT summary FROM events WHERE seq = ?`, eventSeq).Scan(&summary)
	if err == sql.ErrNoRows {
		return "", ErrNotFound
	}
	return summary, err
}

// LastEventType returns type of the latest event.
func (c *Chronicle) LastEventType(ctx context.Context, circleID string) (string, error) {
	var typ string
	err := c.db.QueryRowContext(ctx, `
		SELECT event_type FROM events WHERE circle_id = ? ORDER BY seq DESC LIMIT 1
	`, circleID).Scan(&typ)
	if err == sql.ErrNoRows {
		return "", ErrNotFound
	}
	return typ, err
}

// AssertDayCascadeRemoved is a test helper verifying day metadata gone.
func (c *Chronicle) AssertDayCascadeRemoved(ctx context.Context, circleID, entryDate string) error {
	exists, err := c.DayExists(ctx, circleID, entryDate)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("day still exists")
	}
	var n int
	if err := c.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM day_titles WHERE circle_id = ? AND entry_date = ?
	`, circleID, entryDate).Scan(&n); err != nil {
		return err
	}
	if n != 0 {
		return fmt.Errorf("day_titles remain")
	}
	return nil
}
