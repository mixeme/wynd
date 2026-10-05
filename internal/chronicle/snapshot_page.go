package chronicle

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// Подгрузка ленты, «Сетки» и «Карты» порциями (план 46, C18).
//
// Снимок отдаёт SnapshotPostLimit самых новых; раньше старшее было видно
// только через «Дни». Теперь к порции прилагается курсор — время создания и
// id последней записи порции, — и клиент просит следующую: «старше этого».
// Порядок везде created_at DESC, id DESC: курсор однозначен и при записях,
// созданных в одну наносекунду.

// PageCursor — граница порции: всё строго старше (created_at, id).
type PageCursor struct {
	CreatedAt string
	ID        string
}

// String — непрозрачная строка для API: «created_at|id».
func (p PageCursor) String() string {
	return p.CreatedAt + "|" + p.ID
}

// ParsePageCursor разбирает строку курсора; пустая — первая порция.
func ParsePageCursor(raw string) (*PageCursor, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	i := strings.LastIndex(raw, "|")
	if i <= 0 || i == len(raw)-1 {
		return nil, ErrInvalid
	}
	created, id := raw[:i], raw[i+1:]
	if _, err := time.Parse(time.RFC3339Nano, created); err != nil {
		return nil, ErrInvalid
	}
	return &PageCursor{CreatedAt: created, ID: id}, nil
}

// beforeSQL — условие «старше курсора» для таблицы с псевдонимом alias.
func beforeSQL(alias string, before *PageCursor) (string, []any) {
	if before == nil {
		return "1=1", nil
	}
	return fmt.Sprintf("(%[1]s.created_at < ? OR (%[1]s.created_at = ? AND %[1]s.id < ?))", alias),
		[]any{before.CreatedAt, before.CreatedAt, before.ID}
}

// FeedPage — порция ленты: до SnapshotPostLimit записей старше before
// (nil — самые новые) и курсор следующей порции, если она есть.
func (c *Chronicle) FeedPage(ctx context.Context, circleID, accountID string, before *PageCursor) ([]FeedPost, *PageCursor, error) {
	if err := c.requireReader(ctx, circleID, accountID); err != nil {
		return nil, nil, err
	}
	scope, err := c.newReadScope(ctx, circleID, accountID)
	if err != nil {
		return nil, nil, err
	}
	where, args := beforeSQL("posts", before)
	query := fmt.Sprintf(`
		SELECT id, created_at FROM posts
		WHERE circle_id = ? AND deleted = 0
		  AND %s
		  AND %s
		ORDER BY created_at DESC, id DESC
		LIMIT ?
	`, where, sqlVisibleAt("posts.created_at"))
	all := append([]any{circleID}, args...)
	all = append(all, circleID, accountID, SnapshotPostLimit+1)
	rows, err := c.db.QueryContext(ctx, query, all...)
	if err != nil {
		return nil, nil, err
	}
	var ids []string
	var created []string
	for rows.Next() {
		var id, at string
		if err := rows.Scan(&id, &at); err != nil {
			rows.Close()
			return nil, nil, err
		}
		ids = append(ids, id)
		created = append(created, at)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	var next *PageCursor
	if len(ids) > SnapshotPostLimit {
		ids = ids[:SnapshotPostLimit]
		last := SnapshotPostLimit - 1
		next = &PageCursor{CreatedAt: created[last], ID: ids[last]}
	}
	posts, err := c.buildFeedPosts(ctx, ids, circleID, scope)
	if err != nil {
		return nil, nil, err
	}
	return posts, next, nil
}

// mediaRow — строка «Сетки» или «Карты» с границей своей записи.
type mediaRow struct {
	postID  string
	created string
}

// trimPartialPost: порция обрезана по LIMIT посреди записи — её хвост
// уходит в следующую порцию целиком, чтобы курсор по записи ничего не терял.
// Возвращает, сколько строк оставить, и курсор следующей порции.
func trimPartialPost(rows []mediaRow, limit int) (int, *PageCursor) {
	if len(rows) <= limit {
		return len(rows), nil
	}
	cut := limit
	spill := rows[limit].postID
	for cut > 0 && rows[cut-1].postID == spill {
		cut--
	}
	if cut == 0 {
		// Одна запись больше порции — отдаём как есть, без её хвоста.
		cut = limit
	}
	last := rows[cut-1]
	return cut, &PageCursor{CreatedAt: last.created, ID: last.postID}
}

// GridPage — порция фото «Сетки» (как GridSnapshot) и курсор следующей.
func (c *Chronicle) GridPage(ctx context.Context, circleID, accountID string, before *PageCursor) ([]GridItem, *PageCursor, error) {
	if err := c.requireReader(ctx, circleID, accountID); err != nil {
		return nil, nil, err
	}
	where, args := beforeSQL("p", before)
	all := append([]any{circleID}, args...)
	all = append(all, circleID, accountID, SnapshotPostLimit+1)
	rows, err := c.db.QueryContext(ctx, fmt.Sprintf(`
		SELECT p.id,
			CASE WHEN pm.kind = 'video' AND COALESCE(pm.video_poster_blob_id, '') != ''
				THEN pm.video_poster_blob_id ELSE pm.blob_id END,
			p.entry_date, p.created_at, pm.is_cover,
			CASE WHEN pm.kind = 'video' AND COALESCE(pm.video_poster_blob_id, '') != ''
				THEN 'photo' ELSE pm.kind END
		FROM posts p
		JOIN post_media pm ON pm.post_id = p.id AND pm.kind IN ('photo', 'video')
		WHERE p.circle_id = ? AND p.deleted = 0
		  AND %s
		  AND %s
		ORDER BY p.created_at DESC, p.id DESC, pm.sort_order
		LIMIT ?
	`, where, sqlVisibleAt("p.created_at")), all...)
	if err != nil {
		return nil, nil, err
	}
	var items []GridItem
	var bounds []mediaRow
	for rows.Next() {
		var item GridItem
		var created string
		var cover int
		if err := rows.Scan(&item.PostID, &item.BlobID, &item.EntryDate, &created, &cover, &item.Kind); err != nil {
			rows.Close()
			return nil, nil, err
		}
		item.CreatedAt, _ = parseTime(created)
		item.IsCover = cover == 1
		items = append(items, item)
		bounds = append(bounds, mediaRow{postID: item.PostID, created: created})
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	keep, next := trimPartialPost(bounds, SnapshotPostLimit)
	items = items[:keep]
	seen := make(map[string]bool, len(items))
	out := items[:0]
	for _, item := range items {
		key := item.PostID + ":" + item.BlobID
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, item)
	}
	return out, next, nil
}

// MapPage — порция точек «Карты» (как MapSnapshot) и курсор следующей.
func (c *Chronicle) MapPage(ctx context.Context, circleID, accountID string, before *PageCursor) ([]MapPin, *PageCursor, error) {
	if err := c.requireReader(ctx, circleID, accountID); err != nil {
		return nil, nil, err
	}
	where, args := beforeSQL("p", before)
	all := append([]any{circleID}, args...)
	all = append(all, circleID, accountID, SnapshotPostLimit+1)
	rows, err := c.db.QueryContext(ctx, fmt.Sprintf(`
		SELECT p.id, pm.blob_id, p.entry_date, p.created_at, pm.geo_lat, pm.geo_lng, p.author_name, p.body
		FROM posts p
		JOIN post_media pm ON pm.post_id = p.id
		WHERE p.circle_id = ? AND p.deleted = 0
		  AND pm.geo_lat IS NOT NULL AND pm.geo_lng IS NOT NULL
		  AND %s
		  AND %s
		ORDER BY p.created_at DESC, p.id DESC, pm.sort_order
		LIMIT ?
	`, where, sqlVisibleAt("p.created_at")), all...)
	if err != nil {
		return nil, nil, err
	}
	var pins []MapPin
	var bounds []mediaRow
	for rows.Next() {
		var pin MapPin
		var created string
		if err := rows.Scan(&pin.PostID, &pin.BlobID, &pin.EntryDate, &created, &pin.GeoLat, &pin.GeoLng, &pin.AuthorName, &pin.Body); err != nil {
			rows.Close()
			return nil, nil, err
		}
		pin.CreatedAt, _ = parseTime(created)
		pins = append(pins, pin)
		bounds = append(bounds, mediaRow{postID: pin.PostID, created: created})
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	keep, next := trimPartialPost(bounds, SnapshotPostLimit)
	return pins[:keep], next, nil
}
