package chronicle

import (
	"context"
	"database/sql"
	"time"
)

// CircleSummary is a circle row for the participant list.
type CircleSummary struct {
	ID          string
	Name        string
	Color       string
	Status      MembershipStatus
	Unread      int
	LastReadSeq int64
	LastSummary string
	LastAt      *time.Time
}

// ListAccountCircles returns circles the account belongs to with unread counts.
func (c *Chronicle) ListAccountCircles(ctx context.Context, accountID string) ([]CircleSummary, error) {
	rows, err := c.db.QueryContext(ctx, `
		SELECT c.id, c.name, c.color, m.status
		FROM memberships m
		JOIN circles c ON c.id = m.circle_id
		WHERE m.account_id = ?
		ORDER BY c.name
	`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CircleSummary
	for rows.Next() {
		var s CircleSummary
		var status string
		if err := rows.Scan(&s.ID, &s.Name, &s.Color, &status); err != nil {
			return nil, err
		}
		s.Status = MembershipStatus(status)
		cur, err := c.GetReadCursor(ctx, accountID, s.ID)
		if err != nil {
			return nil, err
		}
		s.LastReadSeq = cur.LastReadSeq
		unread, err := c.UnreadPostCount(ctx, accountID, s.ID)
		if err != nil {
			return nil, err
		}
		s.Unread = unread
		summary, at, ok, err := c.LastVisibleEvent(ctx, s.ID, accountID)
		if err != nil {
			return nil, err
		}
		if ok {
			s.LastSummary = summary
			s.LastAt = &at
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// FeedPost is a post with nested comments and reactions for snapshot APIs.
type FeedPost struct {
	Post      Post
	Media     []PostMedia
	Comments  []Comment
	Reactions []Reaction
}

// FeedSnapshot returns visible posts ordered by created_at descending.
func (c *Chronicle) FeedSnapshot(ctx context.Context, circleID, accountID string) ([]FeedPost, error) {
	if err := c.requireReader(ctx, circleID, accountID); err != nil {
		return nil, err
	}
	rows, err := c.db.QueryContext(ctx, `
		SELECT id FROM posts
		WHERE circle_id = ? AND deleted = 0
		ORDER BY created_at DESC
	`, circleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []FeedPost
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		fp, err := c.loadFeedPost(ctx, id, circleID, accountID)
		if err != nil {
			return nil, err
		}
		if fp != nil {
			out = append(out, *fp)
		}
	}
	return out, rows.Err()
}

// GridItem is a photo tile for the grid view.
type GridItem struct {
	PostID    string
	BlobID    string
	EntryDate string
	CreatedAt time.Time
	IsCover   bool
}

// GridSnapshot returns visible photo media ordered by post created_at descending.
func (c *Chronicle) GridSnapshot(ctx context.Context, circleID, accountID string) ([]GridItem, error) {
	if err := c.requireReader(ctx, circleID, accountID); err != nil {
		return nil, err
	}
	rows, err := c.db.QueryContext(ctx, `
		SELECT p.id, pm.blob_id, p.entry_date, p.created_at, pm.is_cover
		FROM posts p
		JOIN post_media pm ON pm.post_id = p.id AND pm.kind = 'photo'
		WHERE p.circle_id = ? AND p.deleted = 0
		ORDER BY p.created_at DESC, pm.sort_order
	`, circleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []GridItem
	seen := make(map[string]bool)
	for rows.Next() {
		var item GridItem
		var created string
		var cover int
		if err := rows.Scan(&item.PostID, &item.BlobID, &item.EntryDate, &created, &cover); err != nil {
			return nil, err
		}
		item.CreatedAt, _ = parseTime(created)
		item.IsCover = cover == 1
		ok, err := c.CanReadEvent(ctx, circleID, accountID, item.CreatedAt)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		key := item.PostID + ":" + item.BlobID
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, item)
	}
	return out, rows.Err()
}

// MapPin is a geotagged media point for the map view.
type MapPin struct {
	PostID    string
	BlobID    string
	EntryDate string
	CreatedAt time.Time
	GeoLat    float64
	GeoLng    float64
}

// MapSnapshot returns visible geotagged media ordered by post created_at descending.
func (c *Chronicle) MapSnapshot(ctx context.Context, circleID, accountID string) ([]MapPin, error) {
	if err := c.requireReader(ctx, circleID, accountID); err != nil {
		return nil, err
	}
	rows, err := c.db.QueryContext(ctx, `
		SELECT p.id, pm.blob_id, p.entry_date, p.created_at, pm.geo_lat, pm.geo_lng
		FROM posts p
		JOIN post_media pm ON pm.post_id = p.id
		WHERE p.circle_id = ? AND p.deleted = 0
		  AND pm.geo_lat IS NOT NULL AND pm.geo_lng IS NOT NULL
		ORDER BY p.created_at DESC
	`, circleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MapPin
	for rows.Next() {
		var pin MapPin
		var created string
		if err := rows.Scan(&pin.PostID, &pin.BlobID, &pin.EntryDate, &created, &pin.GeoLat, &pin.GeoLng); err != nil {
			return nil, err
		}
		pin.CreatedAt, _ = parseTime(created)
		ok, err := c.CanReadEvent(ctx, circleID, accountID, pin.CreatedAt)
		if err != nil {
			return nil, err
		}
		if ok {
			out = append(out, pin)
		}
	}
	return out, rows.Err()
}

// DaySummary is a day projection row visible to the account.
type DaySummary struct {
	Day                 Day
	PostCount           int
	TitleEditableUntil  *time.Time
	CoverEditableUntil  *time.Time
}

// DaysSnapshot returns days with at least one visible post, ordered by entry_date descending.
func (c *Chronicle) DaysSnapshot(ctx context.Context, circleID, accountID string) ([]DaySummary, error) {
	if err := c.requireReader(ctx, circleID, accountID); err != nil {
		return nil, err
	}
	rows, err := c.db.QueryContext(ctx, `
		SELECT d.circle_id, d.entry_date, d.title, d.cover_post_id, d.cover_blob_id
		FROM days d
		WHERE d.circle_id = ?
		ORDER BY d.entry_date DESC
	`, circleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DaySummary
	for rows.Next() {
		var d Day
		var title, coverPost, coverBlob sql.NullString
		if err := rows.Scan(&d.CircleID, &d.EntryDate, &title, &coverPost, &coverBlob); err != nil {
			return nil, err
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
		count, err := c.visiblePostCountForDay(ctx, circleID, accountID, d.EntryDate)
		if err != nil {
			return nil, err
		}
		if count == 0 {
			continue
		}
		titleUntil, err := c.dayTitleEditableUntil(ctx, circleID, d.EntryDate)
		if err != nil {
			return nil, err
		}
		coverUntil, err := c.dayCoverEditableUntil(ctx, circleID, d.EntryDate)
		if err != nil {
			return nil, err
		}
		out = append(out, DaySummary{
			Day: d, PostCount: count,
			TitleEditableUntil: titleUntil,
			CoverEditableUntil: coverUntil,
		})
	}
	return out, rows.Err()
}

// DayPostsSnapshot returns visible posts for a day ordered by captured_at then created_at.
func (c *Chronicle) DayPostsSnapshot(ctx context.Context, circleID, accountID, entryDate string) ([]FeedPost, error) {
	if err := c.requireReader(ctx, circleID, accountID); err != nil {
		return nil, err
	}
	rows, err := c.db.QueryContext(ctx, `
		SELECT id FROM posts
		WHERE circle_id = ? AND entry_date = ? AND deleted = 0
		ORDER BY datetime(COALESCE(captured_at, created_at)) ASC
	`, circleID, entryDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []FeedPost
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		fp, err := c.loadFeedPost(ctx, id, circleID, accountID)
		if err != nil {
			return nil, err
		}
		if fp != nil {
			out = append(out, *fp)
		}
	}
	return out, rows.Err()
}

// RequireReader checks that account may read content in the circle.
func (c *Chronicle) RequireReader(ctx context.Context, circleID, accountID string) error {
	return c.requireReader(ctx, circleID, accountID)
}

func (c *Chronicle) requireReader(ctx context.Context, circleID, accountID string) error {
	mem, err := c.membership(ctx, c.db, circleID, accountID)
	if err != nil {
		return err
	}
	if mem.Status == StatusGone {
		return ErrForbidden
	}
	spans, err := c.visibilitySpans(ctx, circleID, accountID)
	if err != nil {
		return err
	}
	for _, sp := range spans {
		if sp.CanRead {
			return nil
		}
	}
	return ErrForbidden
}

func (c *Chronicle) visiblePostCountForDay(ctx context.Context, circleID, accountID, entryDate string) (int, error) {
	rows, err := c.db.QueryContext(ctx, `
		SELECT created_at FROM posts
		WHERE circle_id = ? AND entry_date = ? AND deleted = 0
	`, circleID, entryDate)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	var n int
	for rows.Next() {
		var created string
		if err := rows.Scan(&created); err != nil {
			return 0, err
		}
		t, _ := parseTime(created)
		ok, err := c.CanReadEvent(ctx, circleID, accountID, t)
		if err != nil {
			return 0, err
		}
		if ok {
			n++
		}
	}
	return n, rows.Err()
}

func (c *Chronicle) loadFeedPost(ctx context.Context, postID, circleID, accountID string) (*FeedPost, error) {
	post, err := c.loadPost(ctx, c.db, postID)
	if err != nil {
		return nil, err
	}
	if post.Deleted || post.CircleID != circleID {
		return nil, nil
	}
	ok, err := c.CanReadEvent(ctx, circleID, accountID, post.CreatedAt)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, nil
	}
	media, err := c.ListPostMedia(ctx, postID)
	if err != nil {
		return nil, err
	}
	comments, err := c.listCommentsForPost(ctx, postID, circleID, accountID)
	if err != nil {
		return nil, err
	}
	reactions, err := c.listReactionsForPost(ctx, postID, circleID, accountID)
	if err != nil {
		return nil, err
	}
	return &FeedPost{Post: post, Media: media, Comments: comments, Reactions: reactions}, nil
}

func (c *Chronicle) listCommentsForPost(ctx context.Context, postID, circleID, accountID string) ([]Comment, error) {
	rows, err := c.db.QueryContext(ctx, `
		SELECT id, circle_id, post_id, event_seq, identity_id, author_name, body,
			created_at, edit_window_sec, editable_until, deleted
		FROM comments WHERE post_id = ? AND deleted = 0 ORDER BY created_at
	`, postID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanComments(ctx, c, rows, circleID, accountID)
}

func (c *Chronicle) listReactionsForPost(ctx context.Context, postID, circleID, accountID string) ([]Reaction, error) {
	rows, err := c.db.QueryContext(ctx, `
		SELECT id, circle_id, post_id, event_seq, identity_id, author_name, emoji,
			created_at, edit_window_sec, editable_until, deleted
		FROM reactions WHERE post_id = ? AND deleted = 0 ORDER BY created_at
	`, postID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanReactions(ctx, c, rows, circleID, accountID)
}

func scanComments(ctx context.Context, c *Chronicle, rows *sql.Rows, circleID, accountID string) ([]Comment, error) {
	var out []Comment
	for rows.Next() {
		cm, err := scanCommentRow(rows)
		if err != nil {
			return nil, err
		}
		ok, err := c.CanReadEvent(ctx, circleID, accountID, cm.CreatedAt)
		if err != nil {
			return nil, err
		}
		if ok {
			out = append(out, cm)
		}
	}
	return out, rows.Err()
}

func scanReactions(ctx context.Context, c *Chronicle, rows *sql.Rows, circleID, accountID string) ([]Reaction, error) {
	var out []Reaction
	for rows.Next() {
		rx, err := scanReactionRow(rows)
		if err != nil {
			return nil, err
		}
		ok, err := c.CanReadEvent(ctx, circleID, accountID, rx.CreatedAt)
		if err != nil {
			return nil, err
		}
		if ok {
			out = append(out, rx)
		}
	}
	return out, rows.Err()
}

func scanCommentRow(rows *sql.Rows) (Comment, error) {
	var cm Comment
	var body sql.NullString
	var ew sql.NullInt64
	var until sql.NullString
	var deleted int
	var created string
	err := rows.Scan(&cm.ID, &cm.CircleID, &cm.PostID, &cm.EventSeq, &cm.IdentityID, &cm.AuthorName, &body,
		&created, &ew, &until, &deleted)
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

func scanReactionRow(rows *sql.Rows) (Reaction, error) {
	var rx Reaction
	var ew sql.NullInt64
	var until sql.NullString
	var deleted int
	var created string
	err := rows.Scan(&rx.ID, &rx.CircleID, &rx.PostID, &rx.EventSeq, &rx.IdentityID, &rx.AuthorName, &rx.Emoji,
		&created, &ew, &until, &deleted)
	if err != nil {
		return Reaction{}, err
	}
	rx.CreatedAt, _ = parseTime(created)
	rx.EditWindow, _ = editWindowFromSQL(ew)
	rx.EditableUntil, _ = parseEditableUntil(until)
	rx.Deleted = deleted == 1
	return rx, nil
}

type FeedEventSummary struct {
	Seq       int64
	Summary   string
	CreatedAt time.Time
}

type FeedMeta struct {
	Events          []FeedEventSummary
	VisibleFrom     *time.Time
	CircleStartedAt time.Time
}

func (c *Chronicle) FeedMetaForAccount(ctx context.Context, circleID, accountID string) (FeedMeta, error) {
	if err := c.requireReader(ctx, circleID, accountID); err != nil {
		return FeedMeta{}, err
	}
	events, err := c.feedServiceEvents(ctx, circleID, accountID)
	if err != nil {
		return FeedMeta{}, err
	}
	visibleFrom, circleStartedAt, err := c.feedVisibilityBounds(ctx, circleID, accountID)
	if err != nil {
		return FeedMeta{}, err
	}
	return FeedMeta{
		Events:          events,
		VisibleFrom:     visibleFrom,
		CircleStartedAt: circleStartedAt,
	}, nil
}

func (c *Chronicle) feedServiceEvents(ctx context.Context, circleID, accountID string) ([]FeedEventSummary, error) {
	rows, err := c.db.QueryContext(ctx, `
		SELECT seq, summary, created_at FROM events
		WHERE circle_id = ? AND is_service = 1 AND summary != ''
		ORDER BY seq DESC
	`, circleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []FeedEventSummary
	for rows.Next() {
		var ev FeedEventSummary
		var created string
		if err := rows.Scan(&ev.Seq, &ev.Summary, &created); err != nil {
			return nil, err
		}
		ev.CreatedAt, _ = parseTime(created)
		ok, err := c.CanReadEvent(ctx, circleID, accountID, ev.CreatedAt)
		if err != nil {
			return nil, err
		}
		if ok {
			out = append(out, ev)
		}
	}
	return out, rows.Err()
}

func (c *Chronicle) feedVisibilityBounds(ctx context.Context, circleID, accountID string) (*time.Time, time.Time, error) {
	var created string
	err := c.db.QueryRowContext(ctx, `SELECT created_at FROM circles WHERE id = ?`, circleID).Scan(&created)
	if err == sql.ErrNoRows {
		return nil, time.Time{}, ErrNotFound
	}
	if err != nil {
		return nil, time.Time{}, err
	}
	circleStartedAt, _ := parseTime(created)

	spans, err := c.visibilitySpans(ctx, circleID, accountID)
	if err != nil {
		return nil, time.Time{}, err
	}
	var earliest *time.Time
	for _, sp := range spans {
		if !sp.CanRead {
			continue
		}
		if earliest == nil || sp.StartedAt.Before(*earliest) {
			t := sp.StartedAt
			earliest = &t
		}
	}
	if earliest == nil || !earliest.After(circleStartedAt) {
		return nil, circleStartedAt, nil
	}
	return earliest, circleStartedAt, nil
}

func (c *Chronicle) LastVisibleEvent(ctx context.Context, circleID, accountID string) (summary string, at time.Time, ok bool, err error) {
	rows, err := c.db.QueryContext(ctx, `
		SELECT summary, created_at FROM events
		WHERE circle_id = ? AND summary != ''
		ORDER BY seq DESC
	`, circleID)
	if err != nil {
		return "", time.Time{}, false, err
	}
	defer rows.Close()
	for rows.Next() {
		var sum string
		var created string
		if err := rows.Scan(&sum, &created); err != nil {
			return "", time.Time{}, false, err
		}
		t, _ := parseTime(created)
		visible, err := c.CanReadEvent(ctx, circleID, accountID, t)
		if err != nil {
			return "", time.Time{}, false, err
		}
		if visible {
			return sum, t, true, nil
		}
	}
	return "", time.Time{}, false, rows.Err()
}
