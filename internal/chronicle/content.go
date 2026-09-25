package chronicle

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

var allowedReactionKeys = map[string]bool{
	"heart": true, "laugh": true, "surprise": true, "anger": true,
}

func validReactionKey(emoji string) bool {
	return allowedReactionKeys[emoji]
}

// CreatePost publishes a post and updates day projection.
func (c *Chronicle) CreatePost(ctx context.Context, in PostInput) (Post, error) {
	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return Post{}, err
	}
	defer func() { _ = tx.Rollback() }()
	post, err := c.createPostInTx(ctx, tx, in)
	if err != nil {
		return Post{}, err
	}
	if err := tx.Commit(); err != nil {
		return Post{}, err
	}
	return post, nil
}

// CreatePostInTx is like CreatePost but uses an existing transaction.
func (c *Chronicle) CreatePostInTx(ctx context.Context, tx *sql.Tx, in PostInput) (Post, error) {
	return c.createPostInTx(ctx, tx, in)
}

func (c *Chronicle) createPostInTx(ctx context.Context, tx *sql.Tx, in PostInput) (Post, error) {
	entryDate, err := normalizeEntryDate(in.EntryDate)
	if err != nil {
		return Post{}, err
	}
	body := strings.TrimSpace(in.Body)
	if body == "" && !in.AllowEmptyBody {
		return Post{}, ErrInvalid
	}
	if err := checkByteLen(body, MaxTextBytes); err != nil {
		return Post{}, err
	}
	now := utcOrNow(in.Now)

	mem, err := c.requireWriter(ctx, in.CircleID, in.AccountID, now)
	if err != nil {
		return Post{}, err
	}

	window, err := c.circleEditWindow(ctx, tx, in.CircleID)
	if err != nil {
		return Post{}, err
	}
	name, err := c.identityName(ctx, tx, mem.IdentityID)
	if err != nil {
		return Post{}, err
	}

	postID, err := newID()
	if err != nil {
		return Post{}, err
	}

	ev, err := c.appendEvent(ctx, tx, appendEventInput{
		circleID:        in.CircleID,
		eventType:       "post.created",
		isService:       false,
		actorIdentityID: mem.IdentityID,
		actorName:       name,
		targetID:        postID,
		payload: map[string]any{
			"body":        body,
			"entry_date":  entryDate,
			"captured_at": capturedAtPayload(in.CapturedAt),
		},
		summary: summaryPostCreated(name),
		now:     now,
	})
	if err != nil {
		return Post{}, err
	}

	until := window.EditableUntil(now)
	if err := c.insertPost(ctx, tx, postRow{
		id: postID, circleID: in.CircleID, eventSeq: ev.Seq, identityID: mem.IdentityID,
		authorName: name, body: body, entryDate: entryDate, capturedAt: in.CapturedAt,
		createdAt: now, window: window, editableUntil: until,
	}); err != nil {
		return Post{}, err
	}
	if err := c.ensureDay(ctx, tx, in.CircleID, entryDate); err != nil {
		return Post{}, err
	}

	return Post{
		ID: postID, CircleID: in.CircleID, EventSeq: ev.Seq, IdentityID: mem.IdentityID,
		AuthorName: name, Body: body, EntryDate: entryDate, CapturedAt: in.CapturedAt,
		CreatedAt: now, EditWindow: window, EditableUntil: until,
	}, nil
}

// EditPost updates post body and optionally entry_date within edit window.
func (c *Chronicle) EditPost(ctx context.Context, circleID, accountID, postID, body, entryDate string, now time.Time) error {
	body = strings.TrimSpace(body)
	if err := checkByteLen(body, MaxTextBytes); err != nil {
		return err
	}
	now = utcOrNow(now)
	post, err := c.loadPost(ctx, c.db, postID)
	if err != nil {
		return err
	}
	if post.CircleID != circleID {
		return ErrNotFound
	}
	if post.Deleted {
		return ErrInvalid
	}
	mem, err := c.membership(ctx, c.db, circleID, accountID)
	if err != nil {
		return err
	}
	if mem.IdentityID != post.IdentityID {
		return ErrForbidden
	}
	if !post.EditWindow.CanEdit(post.CreatedAt, now) {
		return ErrForbidden
	}
	if body == "" {
		ids, err := c.PostMediaBlobIDs(ctx, postID)
		if err != nil {
			return err
		}
		if len(ids) == 0 {
			return ErrInvalid
		}
	}

	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	oldDate := post.EntryDate
	if entryDate == "" {
		entryDate = post.EntryDate
	} else {
		entryDate, err = normalizeEntryDate(entryDate)
		if err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE posts SET body = ?, entry_date = ? WHERE id = ?
	`, body, entryDate, postID); err != nil {
		return err
	}
	name, err := c.identityName(ctx, tx, mem.IdentityID)
	if err != nil {
		return err
	}
	if entryDate != oldDate {
		if err := c.reconcileDayAfterEntryDateChange(ctx, tx, circleID, oldDate, entryDate); err != nil {
			return err
		}
	}
	if _, err := c.appendEvent(ctx, tx, appendEventInput{
		circleID: circleID, eventType: "post.edited", isService: false,
		actorIdentityID: mem.IdentityID, actorName: name, targetID: postID,
		payload: map[string]any{"body": body, "entry_date": entryDate},
		summary: summaryPostEdited(name), now: now,
	}); err != nil {
		return err
	}
	return tx.Commit()
}

// DeletePost removes a post branch (post, comments, reactions) and scrubs text.
func (c *Chronicle) DeletePost(ctx context.Context, circleID, accountID, postID string, now time.Time) error {
	now = utcOrNow(now)
	post, err := c.loadPost(ctx, c.db, postID)
	if err != nil {
		return err
	}
	if post.CircleID != circleID {
		return ErrNotFound
	}
	if post.Deleted {
		return ErrInvalid
	}
	mem, err := c.membership(ctx, c.db, circleID, accountID)
	if err != nil {
		return err
	}
	if mem.IdentityID != post.IdentityID {
		return ErrForbidden
	}
	if !post.EditWindow.CanEdit(post.CreatedAt, now) {
		return ErrForbidden
	}

	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if err := c.scrubPostBranch(ctx, tx, post); err != nil {
		return err
	}
	if err := c.maybeCollapseDay(ctx, tx, circleID, post.EntryDate); err != nil {
		return err
	}
	if err := c.reconcileDayCoverAfterPostGone(ctx, tx, circleID, post.EntryDate); err != nil {
		return err
	}
	return tx.Commit()
}

func (c *Chronicle) scrubPostBranch(ctx context.Context, tx *sql.Tx, post Post) error {
	if _, err := tx.ExecContext(ctx, `UPDATE posts SET body = NULL, deleted = 1 WHERE id = ?`, post.ID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE comments SET body = NULL, deleted = 1 WHERE post_id = ?`, post.ID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE reactions SET emoji = '', deleted = 1 WHERE post_id = ?`, post.ID); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `
		UPDATE events SET payload = '{}', summary = ''
		WHERE circle_id = ?
		  AND is_service = 0
		  AND (
		    target_id = ?
		    OR target_id IN (SELECT id FROM comments WHERE post_id = ?)
		    OR target_id IN (SELECT id FROM reactions WHERE post_id = ?)
		  )
	`, post.CircleID, post.ID, post.ID, post.ID)
	return err
}

type postRow struct {
	id            string
	circleID      string
	eventSeq      int64
	identityID    string
	authorName    string
	body          string
	entryDate     string
	capturedAt    *time.Time
	createdAt     time.Time
	window        EditWindow
	editableUntil *time.Time
}

func (c *Chronicle) insertPost(ctx context.Context, tx *sql.Tx, row postRow) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO posts (
			id, circle_id, event_seq, identity_id, author_name, body,
			entry_date, captured_at, created_at, edit_window_sec, editable_until
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, row.id, row.circleID, row.eventSeq, row.identityID, row.authorName, row.body,
		row.entryDate, formatCaptured(row.capturedAt), formatTime(row.createdAt),
		editWindowToSQL(row.window), editableUntilToSQL(row.editableUntil))
	return err
}

func (c *Chronicle) loadPost(ctx context.Context, q querier, postID string) (Post, error) {
	var p Post
	var body, captured sql.NullString
	var ew sql.NullInt64
	var until sql.NullString
	var deleted int
	var created string
	err := q.QueryRowContext(ctx, `
		SELECT id, circle_id, event_seq, identity_id, author_name, body,
			entry_date, captured_at, created_at, edit_window_sec, editable_until, deleted
		FROM posts WHERE id = ?
	`, postID).Scan(&p.ID, &p.CircleID, &p.EventSeq, &p.IdentityID, &p.AuthorName, &body,
		&p.EntryDate, &captured, &created, &ew, &until, &deleted)
	if err == sql.ErrNoRows {
		return Post{}, ErrNotFound
	}
	if err != nil {
		return Post{}, err
	}
	if body.Valid {
		p.Body = body.String
	}
	if captured.Valid {
		t, _ := parseTime(captured.String)
		p.CapturedAt = &t
	}
	p.CreatedAt, _ = parseTime(created)
	p.EditWindow, _ = editWindowFromSQL(ew)
	p.EditableUntil, _ = parseEditableUntil(until)
	p.Deleted = deleted == 1
	return p, nil
}

func formatCaptured(t *time.Time) sql.NullString {
	if t == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: formatTime(*t), Valid: true}
}

func capturedAtPayload(t *time.Time) any {
	if t == nil {
		return nil
	}
	return formatTime(*t)
}

func (c *Chronicle) requireWriter(ctx context.Context, circleID, accountID string, now time.Time) (Membership, error) {
	mem, err := c.membership(ctx, c.db, circleID, accountID)
	if err != nil {
		return Membership{}, err
	}
	canWrite, err := c.CanWrite(ctx, circleID, accountID, now)
	if err != nil {
		return Membership{}, err
	}
	if !canWrite {
		return Membership{}, ErrForbidden
	}
	return mem, nil
}

func (c *Chronicle) assertPostInteractive(ctx context.Context, circleID, accountID string, post Post) error {
	ok, err := c.CanReadEvent(ctx, circleID, accountID, post.CreatedAt)
	if err != nil {
		return err
	}
	if !ok {
		return ErrForbidden
	}
	cycle, err := c.GetArchiveCycle(ctx, circleID)
	if err != nil {
		return err
	}
	if !cycle.Active {
		return nil
	}
	cutoff, err := CutoffInstant(cycle.CutoffDate)
	if err != nil {
		return err
	}
	if post.CreatedAt.Before(cutoff) {
		return ErrForbidden
	}
	return nil
}

// CreateComment adds a flat comment with its own edit window snapshot.
func (c *Chronicle) CreateComment(ctx context.Context, in CommentInput) (Comment, error) {
	body := strings.TrimSpace(in.Body)
	if body == "" {
		return Comment{}, ErrInvalid
	}
	if err := checkByteLen(body, MaxTextBytes); err != nil {
		return Comment{}, err
	}
	now := utcOrNow(in.Now)
	mem, err := c.requireWriter(ctx, in.CircleID, in.AccountID, now)
	if err != nil {
		return Comment{}, err
	}
	post, err := c.loadPost(ctx, c.db, in.PostID)
	if err != nil {
		return Comment{}, err
	}
	if post.CircleID != in.CircleID || post.Deleted {
		return Comment{}, ErrInvalid
	}
	if err := c.assertPostInteractive(ctx, in.CircleID, in.AccountID, post); err != nil {
		return Comment{}, err
	}

	window, err := c.circleEditWindow(ctx, c.db, in.CircleID)
	if err != nil {
		return Comment{}, err
	}
	name, err := c.identityName(ctx, c.db, mem.IdentityID)
	if err != nil {
		return Comment{}, err
	}
	commentID, err := newID()
	if err != nil {
		return Comment{}, err
	}

	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return Comment{}, err
	}
	defer func() { _ = tx.Rollback() }()

	ev, err := c.appendEvent(ctx, tx, appendEventInput{
		circleID: in.CircleID, eventType: "comment.created", isService: false,
		actorIdentityID: mem.IdentityID, actorName: name, targetID: commentID,
		payload: map[string]any{"post_id": in.PostID, "body": body},
		summary: summaryCommentCreated(name), now: now,
	})
	if err != nil {
		return Comment{}, err
	}
	until := window.EditableUntil(now)
	_, err = tx.ExecContext(ctx, `
		INSERT INTO comments (
			id, circle_id, post_id, event_seq, identity_id, author_name, body,
			created_at, edit_window_sec, editable_until
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, commentID, in.CircleID, in.PostID, ev.Seq, mem.IdentityID, name, body,
		formatTime(now), editWindowToSQL(window), editableUntilToSQL(until))
	if err != nil {
		return Comment{}, err
	}
	if err := tx.Commit(); err != nil {
		return Comment{}, err
	}
	return Comment{
		ID: commentID, CircleID: in.CircleID, PostID: in.PostID, EventSeq: ev.Seq,
		IdentityID: mem.IdentityID, AuthorName: name, Body: body, CreatedAt: now,
		EditWindow: window, EditableUntil: until,
	}, nil
}

// SetReaction sets one reaction per identity per post.
func (c *Chronicle) SetReaction(ctx context.Context, in ReactionInput) (Reaction, error) {
	if in.Emoji == "" || !validReactionKey(in.Emoji) {
		return Reaction{}, ErrInvalid
	}
	if err := checkLen(in.Emoji, MaxEmojiChars); err != nil {
		return Reaction{}, err
	}
	now := utcOrNow(in.Now)
	mem, err := c.requireWriter(ctx, in.CircleID, in.AccountID, now)
	if err != nil {
		return Reaction{}, err
	}
	post, err := c.loadPost(ctx, c.db, in.PostID)
	if err != nil {
		return Reaction{}, err
	}
	if post.CircleID != in.CircleID || post.Deleted {
		return Reaction{}, ErrInvalid
	}
	if err := c.assertPostInteractive(ctx, in.CircleID, in.AccountID, post); err != nil {
		return Reaction{}, err
	}
	window, err := c.circleEditWindow(ctx, c.db, in.CircleID)
	if err != nil {
		return Reaction{}, err
	}
	name, err := c.identityName(ctx, c.db, mem.IdentityID)
	if err != nil {
		return Reaction{}, err
	}

	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return Reaction{}, err
	}
	defer func() { _ = tx.Rollback() }()

	var existingID string
	err = tx.QueryRowContext(ctx, `
		SELECT id FROM reactions WHERE post_id = ? AND identity_id = ?
	`, in.PostID, mem.IdentityID).Scan(&existingID)
	if err != nil && err != sql.ErrNoRows {
		return Reaction{}, err
	}

	reactionID := existingID
	if reactionID == "" {
		reactionID, err = newID()
		if err != nil {
			return Reaction{}, err
		}
	}

	ev, err := c.appendEvent(ctx, tx, appendEventInput{
		circleID: in.CircleID, eventType: "reaction.set", isService: false,
		actorIdentityID: mem.IdentityID, actorName: name, targetID: reactionID,
		payload: map[string]any{"post_id": in.PostID, "emoji": in.Emoji},
		summary: summaryReactionSet(name, in.Emoji), now: now,
	})
	if err != nil {
		return Reaction{}, err
	}
	until := window.EditableUntil(now)
	if existingID == "" {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO reactions (
				id, circle_id, post_id, event_seq, identity_id, author_name, emoji,
				created_at, edit_window_sec, editable_until
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, reactionID, in.CircleID, in.PostID, ev.Seq, mem.IdentityID, name, in.Emoji,
			formatTime(now), editWindowToSQL(window), editableUntilToSQL(until))
	} else {
		_, err = tx.ExecContext(ctx, `
			UPDATE reactions SET event_seq = ?, emoji = ?, created_at = ?,
				edit_window_sec = ?, editable_until = ?, deleted = 0
			WHERE id = ?
		`, ev.Seq, in.Emoji, formatTime(now), editWindowToSQL(window), editableUntilToSQL(until), reactionID)
	}
	if err != nil {
		return Reaction{}, fmt.Errorf("upsert reaction: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Reaction{}, err
	}
	return Reaction{
		ID: reactionID, CircleID: in.CircleID, PostID: in.PostID, EventSeq: ev.Seq,
		IdentityID: mem.IdentityID, AuthorName: name, Emoji: in.Emoji, CreatedAt: now,
		EditWindow: window, EditableUntil: until,
	}, nil
}

// EditComment updates comment body within its own edit window.
func (c *Chronicle) EditComment(ctx context.Context, circleID, accountID, commentID, body string, now time.Time) error {
	body = strings.TrimSpace(body)
	if err := checkByteLen(body, MaxTextBytes); err != nil {
		return err
	}
	if body == "" {
		return ErrInvalid
	}
	now = utcOrNow(now)
	comment, err := c.loadComment(ctx, c.db, commentID)
	if err != nil {
		return err
	}
	if comment.CircleID != circleID {
		return ErrNotFound
	}
	if comment.Deleted {
		return ErrInvalid
	}
	mem, err := c.membership(ctx, c.db, circleID, accountID)
	if err != nil {
		return err
	}
	if mem.IdentityID != comment.IdentityID {
		return ErrForbidden
	}
	if !comment.EditWindow.CanEdit(comment.CreatedAt, now) {
		return ErrForbidden
	}

	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `UPDATE comments SET body = ? WHERE id = ?`, body, commentID); err != nil {
		return err
	}
	name, err := c.identityName(ctx, tx, mem.IdentityID)
	if err != nil {
		return err
	}
	if _, err := c.appendEvent(ctx, tx, appendEventInput{
		circleID: circleID, eventType: "comment.edited", isService: false,
		actorIdentityID: mem.IdentityID, actorName: name, targetID: commentID,
		payload: map[string]any{"post_id": comment.PostID, "body": body},
		summary: summaryCommentEdited(name), now: now,
	}); err != nil {
		return err
	}
	return tx.Commit()
}

// DeleteComment scrubs a comment without a journal tombstone.
func (c *Chronicle) DeleteComment(ctx context.Context, circleID, accountID, commentID string, now time.Time) error {
	now = utcOrNow(now)
	comment, err := c.loadComment(ctx, c.db, commentID)
	if err != nil {
		return err
	}
	if comment.CircleID != circleID {
		return ErrNotFound
	}
	if comment.Deleted {
		return ErrInvalid
	}
	mem, err := c.membership(ctx, c.db, circleID, accountID)
	if err != nil {
		return err
	}
	if mem.IdentityID != comment.IdentityID {
		return ErrForbidden
	}
	if !comment.EditWindow.CanEdit(comment.CreatedAt, now) {
		return ErrForbidden
	}

	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `UPDATE comments SET body = NULL, deleted = 1 WHERE id = ?`, commentID); err != nil {
		return err
	}
	if err := c.scrubTargetEvents(ctx, tx, circleID, commentID); err != nil {
		return err
	}
	return tx.Commit()
}

// DeleteReaction scrubs a reaction without a journal tombstone.
func (c *Chronicle) DeleteReaction(ctx context.Context, circleID, accountID, reactionID string, now time.Time) error {
	now = utcOrNow(now)
	reaction, err := c.loadReaction(ctx, c.db, reactionID)
	if err != nil {
		return err
	}
	if reaction.CircleID != circleID {
		return ErrNotFound
	}
	if reaction.Deleted {
		return ErrInvalid
	}
	mem, err := c.membership(ctx, c.db, circleID, accountID)
	if err != nil {
		return err
	}
	if mem.IdentityID != reaction.IdentityID {
		return ErrForbidden
	}
	if !reaction.EditWindow.CanEdit(reaction.CreatedAt, now) {
		return ErrForbidden
	}
	post, err := c.loadPost(ctx, c.db, reaction.PostID)
	if err != nil {
		return err
	}
	if err := c.assertPostInteractive(ctx, circleID, accountID, post); err != nil {
		return err
	}

	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `UPDATE reactions SET emoji = '', deleted = 1 WHERE id = ?`, reactionID); err != nil {
		return err
	}
	if err := c.scrubTargetEvents(ctx, tx, circleID, reactionID); err != nil {
		return err
	}
	return tx.Commit()
}

// ReactionIDForPost returns the active reaction id for an identity on a post.
func (c *Chronicle) ReactionIDForPost(ctx context.Context, circleID, postID, identityID string) (string, error) {
	var id string
	err := c.db.QueryRowContext(ctx, `
		SELECT id FROM reactions
		WHERE circle_id = ? AND post_id = ? AND identity_id = ? AND deleted = 0 AND emoji != ''
	`, circleID, postID, identityID).Scan(&id)
	if err == sql.ErrNoRows {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	return id, nil
}

func (c *Chronicle) scrubTargetEvents(ctx context.Context, tx *sql.Tx, circleID, targetID string) error {
	_, err := tx.ExecContext(ctx, `
		UPDATE events SET payload = '{}', summary = ''
		WHERE circle_id = ? AND is_service = 0 AND target_id = ?
	`, circleID, targetID)
	return err
}

func (c *Chronicle) loadComment(ctx context.Context, q querier, commentID string) (Comment, error) {
	var cm Comment
	var body sql.NullString
	var ew sql.NullInt64
	var until sql.NullString
	var deleted int
	var created string
	err := q.QueryRowContext(ctx, `
		SELECT id, circle_id, post_id, event_seq, identity_id, author_name, body,
			created_at, edit_window_sec, editable_until, deleted
		FROM comments WHERE id = ?
	`, commentID).Scan(&cm.ID, &cm.CircleID, &cm.PostID, &cm.EventSeq, &cm.IdentityID, &cm.AuthorName, &body,
		&created, &ew, &until, &deleted)
	if err == sql.ErrNoRows {
		return Comment{}, ErrNotFound
	}
	if err != nil {
		return Comment{}, err
	}
	if body.Valid {
		cm.Body = body.String
	}
	cm.CreatedAt, _ = parseTime(created)
	cm.EditWindow, _ = editWindowFromSQL(ew)
	cm.EditableUntil, _ = parseEditableUntil(until)
	cm.Deleted = deleted == 1
	return cm, nil
}

func (c *Chronicle) loadReaction(ctx context.Context, q querier, reactionID string) (Reaction, error) {
	var r Reaction
	var ew sql.NullInt64
	var until sql.NullString
	var deleted int
	var created string
	err := q.QueryRowContext(ctx, `
		SELECT id, circle_id, post_id, event_seq, identity_id, author_name, emoji,
			created_at, edit_window_sec, editable_until, deleted
		FROM reactions WHERE id = ?
	`, reactionID).Scan(&r.ID, &r.CircleID, &r.PostID, &r.EventSeq, &r.IdentityID, &r.AuthorName, &r.Emoji,
		&created, &ew, &until, &deleted)
	if err == sql.ErrNoRows {
		return Reaction{}, ErrNotFound
	}
	if err != nil {
		return Reaction{}, err
	}
	r.CreatedAt, _ = parseTime(created)
	r.EditWindow, _ = editWindowFromSQL(ew)
	r.EditableUntil, _ = parseEditableUntil(until)
	r.Deleted = deleted == 1
	return r, nil
}

// DeleteServiceEvent is always forbidden.
func (c *Chronicle) DeleteServiceEvent(ctx context.Context, circleID, accountID string, eventSeq int64, now time.Time) error {
	var isService int
	err := c.db.QueryRowContext(ctx, `
		SELECT is_service FROM events WHERE seq = ? AND circle_id = ?
	`, eventSeq, circleID).Scan(&isService)
	if err == sql.ErrNoRows {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if isService == 1 {
		return ErrForbidden
	}
	_ = accountID
	_ = now
	return ErrForbidden
}
