package chronicle

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// ArchiveCycle describes an active quota archive cycle for a circle.
type ArchiveCycle struct {
	Active            bool
	CutoffDate        string
	Deadline          time.Time
	ReminderBeforeSec int64
	CutoffLockedAt    *time.Time
	ReminderSentAt    *time.Time
	CycleStartedAt    *time.Time
}

// MinArchiveWindow — самый короткий срок цикла архивации от момента старта
// или переноса. Срок «вчера» принимался: скачивание сразу отвечало 403,
// письма уходили с мёртвой ссылкой, а ближайший прогон рутины стирал записи
// до отсечки — участники не успевали ничего (план 42, ARC-7).
const MinArchiveWindow = 24 * time.Hour

// CutoffInstant returns UTC midnight on the cutoff date (exclusive upper bound for purge).
func CutoffInstant(cutoffDate string) (time.Time, error) {
	t, err := time.Parse("2006-01-02", cutoffDate)
	if err != nil {
		return time.Time{}, ErrInvalid
	}
	return t.UTC(), nil
}

// GetArchiveCycle loads the current archive cycle for a circle.
func (c *Chronicle) GetArchiveCycle(ctx context.Context, circleID string) (ArchiveCycle, error) {
	var cutoff, deadline, locked, reminderSent, started sql.NullString
	var reminderBefore sql.NullInt64
	err := c.db.QueryRowContext(ctx, `
		SELECT archive_cutoff_date, archive_deadline, archive_reminder_before_sec,
			cutoff_locked_at, archive_reminder_sent_at, archive_cycle_started_at
		FROM circles WHERE id = ?
	`, circleID).Scan(&cutoff, &deadline, &reminderBefore, &locked, &reminderSent, &started)
	if err == sql.ErrNoRows {
		return ArchiveCycle{}, ErrNotFound
	}
	if err != nil {
		return ArchiveCycle{}, err
	}
	if !cutoff.Valid || cutoff.String == "" {
		return ArchiveCycle{Active: false}, nil
	}
	out := ArchiveCycle{
		Active:     true,
		CutoffDate: cutoff.String,
	}
	if deadline.Valid && deadline.String != "" {
		out.Deadline, _ = parseTime(deadline.String)
	}
	if reminderBefore.Valid {
		out.ReminderBeforeSec = reminderBefore.Int64
	}
	if locked.Valid && locked.String != "" {
		t, _ := parseTime(locked.String)
		out.CutoffLockedAt = &t
	}
	if reminderSent.Valid && reminderSent.String != "" {
		t, _ := parseTime(reminderSent.String)
		out.ReminderSentAt = &t
	}
	if started.Valid && started.String != "" {
		t, _ := parseTime(started.String)
		out.CycleStartedAt = &t
	}
	return out, nil
}

// StartArchiveCycle sets cutoff, deadline and reminder interval (owner only).
func (c *Chronicle) StartArchiveCycle(ctx context.Context, circleID, ownerAccountID, cutoffDate string, deadline time.Time, reminderBeforeSec int64, now time.Time) error {
	if err := c.RequireOwner(ctx, circleID, ownerAccountID); err != nil {
		return err
	}
	if _, err := CutoffInstant(cutoffDate); err != nil {
		return err
	}
	if deadline.IsZero() || reminderBeforeSec < 0 {
		return ErrInvalid
	}
	now = utcOrNow(now)
	deadline = deadline.UTC()
	if deadline.Before(now.Add(MinArchiveWindow)) {
		return ErrInvalid
	}

	mem, err := c.membership(ctx, c.db, circleID, ownerAccountID)
	if err != nil {
		return err
	}
	name, err := c.identityName(ctx, c.db, mem.IdentityID)
	if err != nil {
		return err
	}

	cycle, err := c.GetArchiveCycle(ctx, circleID)
	if err != nil {
		return err
	}
	// Unlocked active cycle is edited via MoveCutoff, not restarted.
	// After the first download the cutoff is locked: a different range is a new cycle.
	if cycle.Active && cycle.CutoffLockedAt == nil {
		return ErrInvalid
	}

	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	updated := formatTime(now)
	if _, err := tx.ExecContext(ctx, `
		UPDATE circles SET
			archive_cutoff_date = ?,
			archive_deadline = ?,
			archive_reminder_before_sec = ?,
			cutoff_locked_at = NULL,
			archive_reminder_sent_at = NULL,
			archive_cycle_started_at = ?,
			updated_at = ?
		WHERE id = ?
	`, cutoffDate, formatTime(deadline), reminderBeforeSec, updated, updated, circleID); err != nil {
		return fmt.Errorf("start archive cycle: %w", err)
	}

	if _, err := c.appendEvent(ctx, tx, appendEventInput{
		circleID:        circleID,
		eventType:       "cutoff_set",
		isService:       true,
		actorIdentityID: mem.IdentityID,
		actorName:       name,
		payload: map[string]any{
			"cutoff_date":         cutoffDate,
			"deadline":            formatTime(deadline),
			"reminder_before_sec": reminderBeforeSec,
		},
		summary: summaryCutoffSet(cutoffDate),
		now:     now,
	}); err != nil {
		return err
	}
	if _, err := c.appendEvent(ctx, tx, appendEventInput{
		circleID:        circleID,
		eventType:       "deadline_set",
		isService:       true,
		actorIdentityID: mem.IdentityID,
		actorName:       name,
		payload: map[string]any{
			"deadline":            formatTime(deadline),
			"reminder_before_sec": reminderBeforeSec,
		},
		summary: summaryDeadlineSet(formatDeadlineLabel(deadline)),
		now:     now,
	}); err != nil {
		return err
	}
	return tx.Commit()
}

// MoveCutoff changes cutoff while the cycle is unlocked (owner only).
func (c *Chronicle) MoveCutoff(ctx context.Context, circleID, ownerAccountID, cutoffDate string, now time.Time) error {
	if err := c.RequireOwner(ctx, circleID, ownerAccountID); err != nil {
		return err
	}
	if _, err := CutoffInstant(cutoffDate); err != nil {
		return err
	}
	now = utcOrNow(now)

	cycle, err := c.GetArchiveCycle(ctx, circleID)
	if err != nil {
		return err
	}
	if !cycle.Active {
		return ErrInvalid
	}
	if cycle.CutoffLockedAt != nil {
		return ErrForbidden
	}
	if cycle.CutoffDate == cutoffDate {
		return nil
	}

	mem, err := c.membership(ctx, c.db, circleID, ownerAccountID)
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

	prev := cycle.CutoffDate
	if _, err := tx.ExecContext(ctx, `
		UPDATE circles SET archive_cutoff_date = ?, updated_at = ? WHERE id = ?
	`, cutoffDate, formatTime(now), circleID); err != nil {
		return err
	}
	if _, err := c.appendEvent(ctx, tx, appendEventInput{
		circleID:        circleID,
		eventType:       "cutoff_moved",
		isService:       true,
		actorIdentityID: mem.IdentityID,
		actorName:       name,
		payload: map[string]any{
			"cutoff_date": cutoffDate,
			"previous":    prev,
		},
		summary: summaryCutoffMoved(prev, cutoffDate),
		now:     now,
	}); err != nil {
		return err
	}
	return tx.Commit()
}

// MoveDeadline changes deadline and resets reminder scheduling (owner only).
func (c *Chronicle) MoveDeadline(ctx context.Context, circleID, ownerAccountID string, deadline time.Time, now time.Time) error {
	if err := c.RequireOwner(ctx, circleID, ownerAccountID); err != nil {
		return err
	}
	if deadline.IsZero() {
		return ErrInvalid
	}
	now = utcOrNow(now)
	deadline = deadline.UTC()
	if deadline.Before(now.Add(MinArchiveWindow)) {
		return ErrInvalid
	}

	cycle, err := c.GetArchiveCycle(ctx, circleID)
	if err != nil {
		return err
	}
	if !cycle.Active {
		return ErrInvalid
	}

	mem, err := c.membership(ctx, c.db, circleID, ownerAccountID)
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

	prev := formatTime(cycle.Deadline)
	if _, err := tx.ExecContext(ctx, `
		UPDATE circles SET
			archive_deadline = ?,
			archive_reminder_sent_at = NULL,
			updated_at = ?
		WHERE id = ?
	`, formatTime(deadline), formatTime(now), circleID); err != nil {
		return err
	}
	if _, err := c.appendEvent(ctx, tx, appendEventInput{
		circleID:        circleID,
		eventType:       "deadline_moved",
		isService:       true,
		actorIdentityID: mem.IdentityID,
		actorName:       name,
		payload: map[string]any{
			"deadline": formatTime(deadline),
			"previous": prev,
		},
		summary: summaryDeadlineMoved(formatDeadlineLabel(cycle.Deadline), formatDeadlineLabel(deadline)),
		now:     now,
	}); err != nil {
		return err
	}
	return tx.Commit()
}

// LockCutoff sets cutoff_locked_at on first successful archive download.
//
// Фиксирует только ту отсечку, по которой собран отданный архив: если
// владелец сдвинул её, пока архив собирался и качался, замораживать новую
// нельзя — у участника на руках срез по старой (план 42, ARC-8).
func (c *Chronicle) LockCutoff(ctx context.Context, circleID, cutoffDate string, now time.Time) (bool, error) {
	now = utcOrNow(now)
	res, err := c.db.ExecContext(ctx, `
		UPDATE circles SET cutoff_locked_at = ?, updated_at = ?
		WHERE id = ? AND archive_cutoff_date = ? AND cutoff_locked_at IS NULL
	`, formatTime(now), formatTime(now), circleID, cutoffDate)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// ArchiveSnapshot returns visible posts before cutoff for personal archive export.
func (c *Chronicle) ArchiveSnapshot(ctx context.Context, circleID, accountID, cutoffDate string) ([]FeedPost, error) {
	cutoff, err := CutoffInstant(cutoffDate)
	if err != nil {
		return nil, err
	}
	if err := c.requireReader(ctx, circleID, accountID); err != nil {
		return nil, err
	}
	rows, err := c.db.QueryContext(ctx, `
		SELECT id FROM posts
		WHERE circle_id = ? AND deleted = 0 AND created_at < ?
		ORDER BY created_at DESC
	`, circleID, formatTime(cutoff))
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
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// Один пакет на весь срез: прежний обход звал loadFeedPost на каждую
	// запись, а тот на каждый вызов заново строил отрезки видимости и делал
	// четыре запроса — на круге в две тысячи записей это восемь тысяч
	// запросов на одну сборку архива (аудит 2026-09-22).
	scope, err := c.newReadScope(ctx, circleID, accountID)
	if err != nil {
		return nil, err
	}
	posts, err := c.buildFeedPosts(ctx, ids, circleID, scope)
	if err != nil {
		return nil, err
	}
	out := make([]FeedPost, 0, len(posts))
	for _, fp := range posts {
		out = append(out, filterFeedPostBefore(fp, cutoff))
	}
	return out, nil
}

// ArchiveDayTitles returns the current titles of the snapshot's days that
// were said before the cutoff, keyed by entry date.
//
// Название дня — сказанное: purge стирает его вместе с записями до отсечки,
// поэтому оно обязано попасть в архив, иначе пропадает без копии (план 42,
// ARC-6). Видимость — как у списка дней: название видно, если в дне есть
// видимая читателю запись, поэтому берутся только дни из среза posts.
func (c *Chronicle) ArchiveDayTitles(ctx context.Context, circleID, cutoffDate string, posts []FeedPost) (map[string]string, error) {
	cutoff, err := CutoffInstant(cutoffDate)
	if err != nil {
		return nil, err
	}
	dates := make(map[string]bool, len(posts))
	for _, fp := range posts {
		dates[fp.Post.EntryDate] = true
	}
	out := make(map[string]string)
	if len(dates) == 0 {
		return out, nil
	}
	rows, err := c.db.QueryContext(ctx, `
		SELECT d.entry_date, d.title
		FROM days d
		JOIN day_titles t ON t.event_seq = d.title_event_seq
		WHERE d.circle_id = ? AND d.title IS NOT NULL AND d.title != ''
		  AND t.created_at < ?
	`, circleID, formatTime(cutoff))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var date, title string
		if err := rows.Scan(&date, &title); err != nil {
			return nil, err
		}
		if dates[date] {
			out[date] = title
		}
	}
	return out, rows.Err()
}

func filterFeedPostBefore(fp FeedPost, cutoff time.Time) FeedPost {
	var comments []Comment
	for _, cm := range fp.Comments {
		if cm.CreatedAt.Before(cutoff) {
			comments = append(comments, cm)
		}
	}
	var reactions []Reaction
	for _, rx := range fp.Reactions {
		if rx.CreatedAt.Before(cutoff) {
			reactions = append(reactions, rx)
		}
	}
	fp.Comments = comments
	fp.Reactions = reactions
	return fp
}

// IdentityIDsFromFeed collects identity ids that appear in a snapshot.
func IdentityIDsFromFeed(posts []FeedPost) []string {
	seen := make(map[string]bool)
	var ids []string
	add := func(id string) {
		if id == "" || seen[id] {
			return
		}
		seen[id] = true
		ids = append(ids, id)
	}
	for _, fp := range posts {
		add(fp.Post.IdentityID)
		for _, cm := range fp.Comments {
			add(cm.IdentityID)
		}
		for _, rx := range fp.Reactions {
			add(rx.IdentityID)
		}
	}
	return ids
}

// avatarBatch — сколько лиц спрашивается одним запросом (предел параметров
// SQLite далеко, пачка лишь держит текст запроса коротким).
const avatarBatch = 200

// IdentityAvatarBlobIDs returns current (non-erased) avatar blob ids for identities.
//
// Пачкой, а не запросом на каждое лицо (план 42, ARC-10). Аватар — у текущей
// неудалённой строки имени; если у неё аватара нет, лица в ответе нет, даже
// когда он был у прежнего имени.
func (c *Chronicle) IdentityAvatarBlobIDs(ctx context.Context, identityIDs []string) (map[string]string, error) {
	out := make(map[string]string)
	for start := 0; start < len(identityIDs); start += avatarBatch {
		marks, args := inClause(identityIDs[start:min(start+avatarBatch, len(identityIDs))])
		rows, err := c.db.QueryContext(ctx, `
			SELECT n.identity_id, n.avatar_blob_id
			FROM identity_names n
			WHERE n.identity_id IN (`+marks+`)
			  AND n.erased_at IS NULL
			  AND n.rowid = (
				SELECT n2.rowid FROM identity_names n2
				WHERE n2.identity_id = n.identity_id AND n2.erased_at IS NULL
				ORDER BY n2.effective_at DESC LIMIT 1
			  )
		`, args...)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var id string
			var blobID sql.NullString
			if err := rows.Scan(&id, &blobID); err != nil {
				rows.Close()
				return nil, err
			}
			if blobID.Valid && blobID.String != "" {
				out[id] = blobID.String
			}
		}
		if err := rows.Close(); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// ArchivePersonalStats describes a member's personal archive slice (same as ZIP /media).
type ArchivePersonalStats struct {
	MediaBytes int64
	MediaFiles int
	PostCount  int
}

// EstimateArchivePersonal counts posts and unique media blobs in the visible archive slice.
//
// Считает запросами, а не по собранному срезу: оценка показывается в баннере
// цикла на каждом GET /circles, а срез — это все записи круга до отсечки со
// всеми комментариями и реакциями (аудит 2026-09-22).
func (c *Chronicle) EstimateArchivePersonal(ctx context.Context, circleID, accountID, cutoffDate string) (ArchivePersonalStats, error) {
	cutoff, err := CutoffInstant(cutoffDate)
	if err != nil {
		return ArchivePersonalStats{}, err
	}
	if err := c.requireReader(ctx, circleID, accountID); err != nil {
		return ArchivePersonalStats{}, err
	}
	at := formatTime(cutoff)

	var stats ArchivePersonalStats
	err = c.db.QueryRowContext(ctx, fmt.Sprintf(`
		SELECT COUNT(*)
		FROM posts p
		JOIN memberships m ON m.circle_id = p.circle_id AND m.account_id = ?
		WHERE p.circle_id = ? AND p.deleted = 0 AND p.created_at < ?
		  AND %s
	`, sqlVisibleAtMembership("p.created_at")), accountID, circleID, at).Scan(&stats.PostCount)
	if err != nil {
		return ArchivePersonalStats{}, err
	}
	if stats.PostCount == 0 {
		return stats, nil
	}

	// Файлы среза: вложения видимых записей и аватары лиц, попавших в срез
	// (авторы записей, комментариев и реакций — как в ZIP).
	visiblePost := fmt.Sprintf(`
		SELECT p.id, p.identity_id, p.created_at
		FROM posts p
		JOIN memberships m ON m.circle_id = p.circle_id AND m.account_id = ?
		WHERE p.circle_id = ? AND p.deleted = 0 AND p.created_at < ?
		  AND %s
	`, sqlVisibleAtMembership("p.created_at"))
	query := fmt.Sprintf(`
		WITH vp AS (%s),
		slice_identities AS (
			SELECT identity_id FROM vp
			UNION
			SELECT cm.identity_id FROM comments cm
			JOIN vp ON vp.id = cm.post_id
			JOIN memberships m ON m.circle_id = ? AND m.account_id = ?
			WHERE cm.deleted = 0 AND cm.created_at < ? AND %s
			UNION
			SELECT rx.identity_id FROM reactions rx
			JOIN vp ON vp.id = rx.post_id
			JOIN memberships m ON m.circle_id = ? AND m.account_id = ?
			WHERE rx.deleted = 0 AND rx.created_at < ? AND %s
		),
		slice_blobs AS (
			SELECT pm.blob_id AS id FROM post_media pm JOIN vp ON vp.id = pm.post_id
			UNION
			SELECT pm.audio_cover_blob_id FROM post_media pm JOIN vp ON vp.id = pm.post_id
			 WHERE pm.audio_cover_blob_id IS NOT NULL AND pm.audio_cover_blob_id != ''
			UNION
			SELECT pm.video_poster_blob_id FROM post_media pm JOIN vp ON vp.id = pm.post_id
			 WHERE pm.video_poster_blob_id IS NOT NULL AND pm.video_poster_blob_id != ''
			UNION
			SELECT cmm.blob_id FROM comment_media cmm
			JOIN comments cm ON cm.id = cmm.comment_id
			JOIN vp ON vp.id = cm.post_id
			JOIN memberships m ON m.circle_id = ? AND m.account_id = ?
			WHERE cm.deleted = 0 AND cm.created_at < ? AND %s
			UNION
			SELECT n.avatar_blob_id FROM identity_names n
			JOIN slice_identities si ON si.identity_id = n.identity_id
			WHERE n.erased_at IS NULL AND n.avatar_blob_id IS NOT NULL
			  AND n.effective_at = (
				SELECT n2.effective_at FROM identity_names n2
				WHERE n2.identity_id = n.identity_id AND n2.erased_at IS NULL
				ORDER BY n2.effective_at DESC LIMIT 1
			  )
		)
		SELECT COUNT(*), COALESCE(SUM(b.size_bytes), 0)
		FROM blobs b
		JOIN slice_blobs sb ON sb.id = b.id
		WHERE b.status = 'complete'
	`, visiblePost,
		sqlVisibleAtMembership("cm.created_at"),
		sqlVisibleAtMembership("rx.created_at"),
		sqlVisibleAtMembership("cm.created_at"))
	args := []any{
		accountID, circleID, at, // vp
		circleID, accountID, at, // комментарии
		circleID, accountID, at, // реакции
		circleID, accountID, at, // вложения комментариев
	}
	if err := c.db.QueryRowContext(ctx, query, args...).Scan(&stats.MediaFiles, &stats.MediaBytes); err != nil {
		return ArchivePersonalStats{}, err
	}
	return stats, nil
}

// CirclesDueForArchivePurge returns circle ids whose deadline has passed.
func (c *Chronicle) CirclesDueForArchivePurge(ctx context.Context, now time.Time) ([]string, error) {
	now = utcOrNow(now)
	rows, err := c.db.QueryContext(ctx, `
		SELECT id, archive_deadline FROM circles
		WHERE archive_cutoff_date IS NOT NULL
		  AND archive_deadline IS NOT NULL
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id, deadline string
		if err := rows.Scan(&id, &deadline); err != nil {
			return nil, err
		}
		t, err := parseTime(deadline)
		if err != nil {
			continue
		}
		if !t.After(now) {
			ids = append(ids, id)
		}
	}
	return ids, rows.Err()
}

// CirclesDueForArchiveReminder returns circles needing reminder emails.
func (c *Chronicle) CirclesDueForArchiveReminder(ctx context.Context, now time.Time) ([]string, error) {
	now = utcOrNow(now)
	rows, err := c.db.QueryContext(ctx, `
		SELECT id, archive_deadline, archive_reminder_before_sec FROM circles
		WHERE archive_cutoff_date IS NOT NULL
		  AND archive_deadline IS NOT NULL
		  AND archive_reminder_before_sec IS NOT NULL
		  AND archive_reminder_sent_at IS NULL
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id, deadline string
		var before int64
		if err := rows.Scan(&id, &deadline, &before); err != nil {
			return nil, err
		}
		t, err := parseTime(deadline)
		if err != nil {
			continue
		}
		if !t.Add(-time.Duration(before) * time.Second).After(now) {
			ids = append(ids, id)
		}
	}
	return ids, rows.Err()
}

// MarkArchiveReminderSent records that the reminder email was sent.
func (c *Chronicle) MarkArchiveReminderSent(ctx context.Context, circleID string, now time.Time) error {
	_, err := c.db.ExecContext(ctx, `
		UPDATE circles SET archive_reminder_sent_at = ? WHERE id = ?
	`, formatTime(utcOrNow(now)), circleID)
	return err
}

// CircleMemberAccountIDs returns accounts to signal about new activity: only
// active members. Вышедший с доступом нового не видит, и сигнал «в круге
// что-то появилось» ему не положен (аудит 2026-09-22).
func (c *Chronicle) CircleMemberAccountIDs(ctx context.Context, circleID string) ([]string, error) {
	rows, err := c.db.QueryContext(ctx, `
		SELECT account_id FROM memberships
		WHERE circle_id = ? AND status = 'active'
		ORDER BY account_id
	`, circleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// CircleMemberEmails returns member emails for active archive notifications.
func (c *Chronicle) CircleMemberEmails(ctx context.Context, circleID string) ([]string, error) {
	rows, err := c.db.QueryContext(ctx, `
		SELECT DISTINCT a.email
		FROM memberships m
		JOIN accounts a ON a.id = m.account_id
		WHERE m.circle_id = ? AND m.status != 'gone'
		ORDER BY a.email
	`, circleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var email string
		if err := rows.Scan(&email); err != nil {
			return nil, err
		}
		out = append(out, email)
	}
	return out, rows.Err()
}

// PurgeBeforeCutoff deletes said content and media before cutoff; structural events remain.
// Returns blob ids that were detached for GC.
func (c *Chronicle) PurgeBeforeCutoff(ctx context.Context, circleID, cutoffDate string, now time.Time) ([]string, error) {
	cutoff, err := CutoffInstant(cutoffDate)
	if err != nil {
		return nil, err
	}
	now = utcOrNow(now)

	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	rows, err := tx.QueryContext(ctx, `
		SELECT id FROM posts
		WHERE circle_id = ? AND deleted = 0 AND created_at < ?
	`, circleID, formatTime(cutoff))
	if err != nil {
		return nil, err
	}
	var postIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		postIDs = append(postIDs, id)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}

	affectedDays := make(map[string]bool)
	var blobIDs []string
	seenBlobs := make(map[string]bool)
	for _, postID := range postIDs {
		post, err := c.loadPost(ctx, tx, postID)
		if err != nil {
			return nil, err
		}
		affectedDays[post.EntryDate] = true
		ids, err := c.purgePostBranch(ctx, tx, post)
		if err != nil {
			return nil, err
		}
		for _, id := range ids {
			if !seenBlobs[id] {
				seenBlobs[id] = true
				blobIDs = append(blobIDs, id)
			}
		}
	}

	// Scrub said day events before cutoff.
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM events
		WHERE circle_id = ?
		  AND is_service = 0
		  AND created_at < ?
		  AND event_type IN ('day.titled', 'day.cover_set', 'day.title_cleared', 'day.cover_cleared')
	`, circleID, formatTime(cutoff)); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM day_titles WHERE circle_id = ? AND created_at < ?
	`, circleID, formatTime(cutoff)); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM day_covers WHERE circle_id = ? AND created_at < ?
	`, circleID, formatTime(cutoff)); err != nil {
		return nil, err
	}

	for entryDate := range affectedDays {
		if err := c.maybeCollapseDay(ctx, tx, circleID, entryDate); err != nil {
			return nil, err
		}
		if err := c.reconcileDayCoverAfterPostGone(ctx, tx, circleID, entryDate); err != nil {
			return nil, err
		}
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE circles SET
			archive_cutoff_date = NULL,
			archive_deadline = NULL,
			archive_reminder_before_sec = NULL,
			cutoff_locked_at = NULL,
			archive_reminder_sent_at = NULL,
			archive_cycle_started_at = NULL,
			updated_at = ?
		WHERE id = ?
	`, formatTime(now), circleID); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return blobIDs, nil
}

func (c *Chronicle) purgePostBranch(ctx context.Context, tx *sql.Tx, post Post) ([]string, error) {
	blobIDs, err := c.postMediaBlobIDsTx(ctx, tx, post.ID)
	if err != nil {
		return nil, err
	}
	if err := c.scrubPostBranch(ctx, tx, post); err != nil {
		return nil, err
	}
	if err := c.dropPostMediaInTx(ctx, tx, post.ID); err != nil {
		return nil, err
	}
	commentBlobs, err := c.takeCommentMediaTx(ctx, tx, commentMediaOfPost, post.ID)
	if err != nil {
		return nil, err
	}
	return append(blobIDs, commentBlobs...), nil
}

func (c *Chronicle) postMediaBlobIDsTx(ctx context.Context, tx *sql.Tx, postID string) ([]string, error) {
	rows, err := tx.QueryContext(ctx, postMediaBlobIDsSQL, postID, postID, postID)
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
