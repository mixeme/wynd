package chronicle

import (
	"context"
	"database/sql"
	"fmt"
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

type lastVisibleEvent struct {
	summary string
	at      time.Time
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
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, nil
	}

	cursors, err := c.batchReadCursors(ctx, accountID)
	if err != nil {
		return nil, err
	}
	unread, err := c.batchUnreadPostCounts(ctx, accountID)
	if err != nil {
		return nil, err
	}
	lastEvents, err := c.batchLastVisibleEvents(ctx, accountID)
	if err != nil {
		return nil, err
	}

	for i := range out {
		id := out[i].ID
		out[i].LastReadSeq = cursors[id]
		out[i].Unread = unread[id]
		if ev, ok := lastEvents[id]; ok {
			out[i].LastSummary = ev.summary
			at := ev.at
			out[i].LastAt = &at
		}
	}
	return out, nil
}

func (c *Chronicle) batchReadCursors(ctx context.Context, accountID string) (map[string]int64, error) {
	rows, err := c.db.QueryContext(ctx, `
		SELECT circle_id, last_read_seq FROM read_cursors
		WHERE account_id = ?
	`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]int64)
	for rows.Next() {
		var circleID string
		var seq int64
		if err := rows.Scan(&circleID, &seq); err != nil {
			return nil, err
		}
		out[circleID] = seq
	}
	return out, rows.Err()
}

func (c *Chronicle) batchUnreadPostCounts(ctx context.Context, accountID string) (map[string]int, error) {
	rows, err := c.db.QueryContext(ctx, fmt.Sprintf(`
		SELECT p.circle_id, COUNT(*)
		FROM posts p
		JOIN memberships m ON m.circle_id = p.circle_id AND m.account_id = ?
		LEFT JOIN read_cursors rc ON rc.account_id = m.account_id AND rc.circle_id = p.circle_id
		WHERE p.deleted = 0
		  AND p.event_seq > COALESCE(rc.last_read_seq, 0)
		  AND %s
		GROUP BY p.circle_id
	`, sqlVisibleAtMembership("p.created_at")), accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]int)
	for rows.Next() {
		var circleID string
		var n int
		if err := rows.Scan(&circleID, &n); err != nil {
			return nil, err
		}
		out[circleID] = n
	}
	return out, rows.Err()
}

func (c *Chronicle) batchLastVisibleEvents(ctx context.Context, accountID string) (map[string]lastVisibleEvent, error) {
	rows, err := c.db.QueryContext(ctx, fmt.Sprintf(`
		SELECT circle_id, summary, created_at FROM (
			SELECT e.circle_id, e.summary, e.created_at,
				ROW_NUMBER() OVER (PARTITION BY e.circle_id ORDER BY e.seq DESC) AS rn
			FROM events e
			JOIN memberships m ON m.circle_id = e.circle_id AND m.account_id = ?
			WHERE e.summary != ''
			  AND %s
		) t
		WHERE rn = 1
	`, sqlVisibleAtMembership("e.created_at")), accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]lastVisibleEvent)
	for rows.Next() {
		var circleID, summary, created string
		if err := rows.Scan(&circleID, &summary, &created); err != nil {
			return nil, err
		}
		at, err := parseTime(created)
		if err != nil {
			return nil, fmt.Errorf("last visible event created_at: %w", err)
		}
		out[circleID] = lastVisibleEvent{summary: summary, at: at}
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
	scope, err := c.newReadScope(ctx, circleID, accountID)
	if err != nil {
		return nil, err
	}
	ids, err := c.visiblePostIDs(ctx, circleID, accountID, "1=1", "created_at DESC", SnapshotPostLimit)
	if err != nil {
		return nil, err
	}
	return c.buildFeedPosts(ctx, ids, circleID, scope)
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
	rows, err := c.db.QueryContext(ctx, fmt.Sprintf(`
		SELECT p.id, pm.blob_id, p.entry_date, p.created_at, pm.is_cover
		FROM posts p
		JOIN post_media pm ON pm.post_id = p.id AND pm.kind = 'photo'
		WHERE p.circle_id = ? AND p.deleted = 0
		  AND %s
		ORDER BY p.created_at DESC, pm.sort_order
		LIMIT ?
	`, sqlVisibleAt("p.created_at")), circleID, circleID, accountID, SnapshotPostLimit)
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
	PostID     string
	BlobID     string
	EntryDate  string
	CreatedAt  time.Time
	GeoLat     float64
	GeoLng     float64
	AuthorName string
	Body       string
}

// MapSnapshot returns visible geotagged media ordered by post created_at descending.
func (c *Chronicle) MapSnapshot(ctx context.Context, circleID, accountID string) ([]MapPin, error) {
	if err := c.requireReader(ctx, circleID, accountID); err != nil {
		return nil, err
	}
	rows, err := c.db.QueryContext(ctx, fmt.Sprintf(`
		SELECT p.id, pm.blob_id, p.entry_date, p.created_at, pm.geo_lat, pm.geo_lng, p.author_name, p.body
		FROM posts p
		JOIN post_media pm ON pm.post_id = p.id
		WHERE p.circle_id = ? AND p.deleted = 0
		  AND pm.geo_lat IS NOT NULL AND pm.geo_lng IS NOT NULL
		  AND %s
		ORDER BY p.created_at DESC
		LIMIT ?
	`, sqlVisibleAt("p.created_at")), circleID, circleID, accountID, SnapshotPostLimit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MapPin
	for rows.Next() {
		var pin MapPin
		var created string
		if err := rows.Scan(&pin.PostID, &pin.BlobID, &pin.EntryDate, &created, &pin.GeoLat, &pin.GeoLng, &pin.AuthorName, &pin.Body); err != nil {
			return nil, err
		}
		pin.CreatedAt, _ = parseTime(created)
		out = append(out, pin)
	}
	return out, rows.Err()
}

// DaySummary is a day projection row visible to the account.
type DaySummary struct {
	Day                Day
	PostCount          int
	TitleEditableUntil *time.Time
	CoverEditableUntil *time.Time
}

// DaysSnapshot returns days with at least one visible post, ordered by entry_date descending.
//
// Три запроса на весь экран, а не 3N+1 (REF-6, CHR-2): дни, счёт видимых
// записей по дням и сроки правки названия и обложки. Прежний обход считал
// записи дня по одной и на каждую спрашивал CanReadEvent отдельным запросом,
// то есть стоил тем дороже, чем длиннее круг.
func (c *Chronicle) DaysSnapshot(ctx context.Context, circleID, accountID string) ([]DaySummary, error) {
	if err := c.requireReader(ctx, circleID, accountID); err != nil {
		return nil, err
	}
	counts, err := c.visiblePostCountsByDay(ctx, circleID, accountID)
	if err != nil {
		return nil, err
	}
	if len(counts) == 0 {
		return nil, nil
	}
	titleUntil, coverUntil, err := c.dayEditableUntils(ctx, circleID)
	if err != nil {
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
		count := counts[d.EntryDate]
		if count == 0 {
			continue
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
		out = append(out, DaySummary{
			Day: d, PostCount: count,
			TitleEditableUntil: titleUntil[d.EntryDate],
			CoverEditableUntil: coverUntil[d.EntryDate],
		})
	}
	return out, rows.Err()
}

// visiblePostCountsByDay считает видимые записи сразу по всем дням круга.
func (c *Chronicle) visiblePostCountsByDay(ctx context.Context, circleID, accountID string) (map[string]int, error) {
	rows, err := c.db.QueryContext(ctx, fmt.Sprintf(`
		SELECT p.entry_date, COUNT(*)
		FROM posts p
		JOIN memberships m ON m.circle_id = p.circle_id AND m.account_id = ?
		WHERE p.circle_id = ? AND p.deleted = 0
		  AND %s
		GROUP BY p.entry_date
	`, sqlVisibleAtMembership("p.created_at")), accountID, circleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]int)
	for rows.Next() {
		var date string
		var n int
		if err := rows.Scan(&date, &n); err != nil {
			return nil, err
		}
		out[date] = n
	}
	return out, rows.Err()
}

// dayEditableUntils отдаёт сроки правки названия и обложки по дням круга —
// по последней строке сказанного на каждый день.
func (c *Chronicle) dayEditableUntils(ctx context.Context, circleID string) (titles, covers map[string]*time.Time, err error) {
	titles = make(map[string]*time.Time)
	covers = make(map[string]*time.Time)
	rows, err := c.db.QueryContext(ctx, `
		SELECT 'title', t.entry_date, t.editable_until
		FROM day_titles t
		WHERE t.circle_id = ?
		  AND t.event_seq = (
			SELECT t2.event_seq FROM day_titles t2
			WHERE t2.circle_id = t.circle_id AND t2.entry_date = t.entry_date
			ORDER BY t2.created_at DESC, t2.event_seq DESC LIMIT 1
		  )
		UNION ALL
		SELECT 'cover', dc.entry_date, dc.editable_until
		FROM day_covers dc
		WHERE dc.circle_id = ?
		  AND dc.event_seq = (
			SELECT dc2.event_seq FROM day_covers dc2
			WHERE dc2.circle_id = dc.circle_id AND dc2.entry_date = dc.entry_date
			ORDER BY dc2.created_at DESC, dc2.event_seq DESC LIMIT 1
		  )
	`, circleID, circleID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var kind, date string
		var until sql.NullString
		if err := rows.Scan(&kind, &date, &until); err != nil {
			return nil, nil, err
		}
		parsed, err := parseEditableUntil(until)
		if err != nil {
			return nil, nil, err
		}
		if kind == "title" {
			titles[date] = parsed
			continue
		}
		covers[date] = parsed
	}
	return titles, covers, rows.Err()
}

// DayPostsSnapshot returns visible posts for a day ordered by captured_at then created_at.
func (c *Chronicle) DayPostsSnapshot(ctx context.Context, circleID, accountID, entryDate string) ([]FeedPost, error) {
	if err := c.requireReader(ctx, circleID, accountID); err != nil {
		return nil, err
	}
	scope, err := c.newReadScope(ctx, circleID, accountID)
	if err != nil {
		return nil, err
	}
	ids, err := c.visiblePostIDs(ctx, circleID, accountID, "entry_date = ?", "datetime(COALESCE(captured_at, created_at)) ASC", SnapshotPostLimit, entryDate)
	if err != nil {
		return nil, err
	}
	return c.buildFeedPosts(ctx, ids, circleID, scope)
}

// RequireReader checks that account may read content in the circle.
func (c *Chronicle) RequireReader(ctx context.Context, circleID, accountID string) error {
	return c.requireReader(ctx, circleID, accountID)
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

// FeedMetaForAccount returns the feed frame for a reader: visible service events and the visibility bounds.
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

// LastVisibleEvent returns the newest summarized event the account may read.
// Only a test oracle for the batch ListAccountCircles now (server-reference, «Legacy и сроки снятия»).
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
