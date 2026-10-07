package chronicle

import (
	"context"
	"database/sql"
	"strings"
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
	// Последняя строка с пустым названием — «убрали название».
	if err == sql.ErrNoRows || (err == nil && title.String == "") {
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
	// Последняя строка без файла — «убрали обложку».
	if err == sql.ErrNoRows || (err == nil && blobID.String == "") {
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

// Название и обложка дня — свой род записи: не правится и не стирается, окна
// правок у него нет (столбцы окна в таблицах остались от прежней модели и
// пишутся пустыми). Сменить или убрать — новая запись, действует последняя.
// Но запись зависимая: уходит из журнала вместе с днём, а обложка — ещё и
// вместе с записью, чей файл выбран.

// SetDayTitle sets day title (last-write-wins projection).
func (c *Chronicle) SetDayTitle(ctx context.Context, in DayTitleInput) error {
	// Как у записи и комментария: в базу идёт обрезанный текст, а название
	// из одних пробелов — не название (аудит 2026-09-22).
	in.Title = strings.TrimSpace(in.Title)
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
	name, err := c.identityName(ctx, c.db, mem.IdentityID)
	if err != nil {
		return err
	}
	g := c.identityGender(ctx, c.db, mem.IdentityID)

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
		summary: summaryDayTitled(g, name, in.Title, in.EntryDate), now: now,
	})
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO day_titles (
			circle_id, entry_date, event_seq, identity_id, title, created_at,
			edit_window_sec, editable_until
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, in.CircleID, in.EntryDate, ev.Seq, mem.IdentityID, in.Title, formatTime(now), nil, nil)
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
	okBlob, err := c.DayCoverBlobOnPost(ctx, in.PostID, in.BlobID)
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
	name, err := c.identityName(ctx, c.db, mem.IdentityID)
	if err != nil {
		return err
	}
	g := c.identityGender(ctx, c.db, mem.IdentityID)

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
		summary: summaryDayCoverSet(g, name, in.EntryDate), now: now,
	})
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO day_covers (
			circle_id, entry_date, event_seq, identity_id, post_id, blob_id, created_at,
			edit_window_sec, editable_until
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, in.CircleID, in.EntryDate, ev.Seq, mem.IdentityID, in.PostID, in.BlobID, formatTime(now), nil, nil)
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

// daySaidAuthor — кто вправе убрать название или обложку дня: тот, кто сейчас
// пишет в круг и у кого есть запись за этот день.
func (c *Chronicle) daySaidAuthor(ctx context.Context, circleID, accountID, entryDate string, now time.Time) (identityID, name string, g Gender, err error) {
	mem, err := c.requireWriter(ctx, circleID, accountID, now)
	if err != nil {
		return "", "", GenderNone, err
	}
	ok, err := c.hasPostForDay(ctx, c.db, circleID, accountID, entryDate)
	if err != nil {
		return "", "", GenderNone, err
	}
	if !ok {
		return "", "", GenderNone, ErrForbidden
	}
	name, err = c.identityName(ctx, c.db, mem.IdentityID)
	if err != nil {
		return "", "", GenderNone, err
	}
	return mem.IdentityID, name, c.identityGender(ctx, c.db, mem.IdentityID), nil
}

// ClearDayTitle records that the day title was removed: a journal entry of its own,
// stored as a title row with an empty title. Earlier titles stay in the journal.
func (c *Chronicle) ClearDayTitle(ctx context.Context, circleID, accountID, entryDate string, now time.Time) error {
	now = utcOrNow(now)
	identityID, name, g, err := c.daySaidAuthor(ctx, circleID, accountID, entryDate, now)
	if err != nil {
		return err
	}

	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	var title sql.NullString
	err = tx.QueryRowContext(ctx, `
		SELECT title FROM days WHERE circle_id = ? AND entry_date = ?
	`, circleID, entryDate).Scan(&title)
	if err == sql.ErrNoRows || (err == nil && title.String == "") {
		return ErrInvalid
	}
	if err != nil {
		return err
	}

	ev, err := c.appendEvent(ctx, tx, appendEventInput{
		circleID: circleID, eventType: "day.title_cleared", isService: false,
		actorIdentityID: identityID, actorName: name,
		payload: map[string]any{"entry_date": entryDate},
		summary: summaryDayTitleCleared(g, name, entryDate), now: now,
	})
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO day_titles (circle_id, entry_date, event_seq, identity_id, title, created_at)
		VALUES (?, ?, ?, ?, '', ?)
	`, circleID, entryDate, ev.Seq, identityID, formatTime(now)); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE days SET title = NULL, title_event_seq = NULL WHERE circle_id = ? AND entry_date = ?
	`, circleID, entryDate); err != nil {
		return err
	}
	return tx.Commit()
}

// ClearDayCover records that the day cover was removed: a journal entry of its own,
// stored as a cover row with an empty blob. It hangs on the post of the cover it
// removes and leaves the journal with that post, like the cover itself.
func (c *Chronicle) ClearDayCover(ctx context.Context, circleID, accountID, entryDate string, now time.Time) error {
	now = utcOrNow(now)
	identityID, name, g, err := c.daySaidAuthor(ctx, circleID, accountID, entryDate, now)
	if err != nil {
		return err
	}

	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	var postID, blobID string
	err = tx.QueryRowContext(ctx, `
		SELECT dc.post_id, dc.blob_id
		FROM day_covers dc
		JOIN posts p ON p.id = dc.post_id AND p.deleted = 0
		WHERE dc.circle_id = ? AND dc.entry_date = ?
		ORDER BY dc.created_at DESC, dc.event_seq DESC LIMIT 1
	`, circleID, entryDate).Scan(&postID, &blobID)
	if err == sql.ErrNoRows || (err == nil && blobID == "") {
		return ErrInvalid
	}
	if err != nil {
		return err
	}

	ev, err := c.appendEvent(ctx, tx, appendEventInput{
		circleID: circleID, eventType: "day.cover_cleared", isService: false,
		actorIdentityID: identityID, actorName: name,
		payload: map[string]any{"entry_date": entryDate},
		summary: summaryDayCoverCleared(g, name, entryDate), now: now,
	})
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO day_covers (circle_id, entry_date, event_seq, identity_id, post_id, blob_id, created_at)
		VALUES (?, ?, ?, ?, ?, '', ?)
	`, circleID, entryDate, ev.Seq, identityID, postID, formatTime(now)); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE days SET cover_post_id = NULL, cover_blob_id = NULL, cover_event_seq = NULL
		WHERE circle_id = ? AND entry_date = ?
	`, circleID, entryDate); err != nil {
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
