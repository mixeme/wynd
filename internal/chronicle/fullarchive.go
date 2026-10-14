package chronicle

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// Полный архив круга (план 51, A1): весь круг — строки базы, файлы и
// читаемая лента — без отбора по видимости. Это копия владельца, из неё круг
// оживляют на любом сервере, поэтому берётся всё, что составляет сам круг, и
// ничего из его жизни на этом сервере (приглашения, курсоры, уведомления).

// FullArchiveTable — строки одной таблицы круга как есть, по колонкам схемы.
type FullArchiveTable struct {
	Name    string
	Columns []string
	Rows    [][]any
}

// FullArchiveData — машинная часть полного архива, снятая одной читающей
// транзакцией: строки согласованы между собой на момент SnapshotAt.
type FullArchiveData struct {
	CircleID   string
	CircleName string
	Color      string
	SnapshotAt time.Time
	Tables     []FullArchiveTable
	// BlobIDs — все файлы круга: медиа записей и комментариев, обложки звука
	// и кадры видео, обложки дней, аватары.
	BlobIDs []string
	Posts   int
}

// fullArchiveQueries — что из базы составляет круг. Порядок — порядок
// зависимостей: оживление вставляет таблицы в этом же порядке.
var fullArchiveQueries = []struct{ name, query string }{
	{"circles", `SELECT * FROM circles WHERE id = ?`},
	{"identities", `SELECT * FROM identities WHERE circle_id = ?`},
	{"identity_names", `SELECT * FROM identity_names WHERE identity_id IN (SELECT id FROM identities WHERE circle_id = ?)`},
	{"memberships", `SELECT * FROM memberships WHERE circle_id = ?`},
	{"membership_spans", `SELECT * FROM membership_spans WHERE membership_id IN (SELECT id FROM memberships WHERE circle_id = ?)`},
	{"events", `SELECT * FROM events WHERE circle_id = ? ORDER BY seq`},
	{"posts", `SELECT * FROM posts WHERE circle_id = ?`},
	{"post_media", `SELECT * FROM post_media WHERE post_id IN (SELECT id FROM posts WHERE circle_id = ?)`},
	{"comments", `SELECT * FROM comments WHERE circle_id = ?`},
	{"comment_media", `SELECT * FROM comment_media WHERE comment_id IN (SELECT id FROM comments WHERE circle_id = ?)`},
	{"reactions", `SELECT * FROM reactions WHERE circle_id = ?`},
	{"days", `SELECT * FROM days WHERE circle_id = ?`},
	{"day_titles", `SELECT * FROM day_titles WHERE circle_id = ?`},
	{"day_covers", `SELECT * FROM day_covers WHERE circle_id = ?`},
	// Почта — чтобы при оживлении позвать людей (6.29: «имена, фото и почта
	// каждого»). Остального об учётной записи круг не знает.
	{"accounts", `SELECT id, email FROM accounts WHERE id IN (
		SELECT account_id FROM memberships WHERE circle_id = ?
		UNION SELECT account_id FROM identities WHERE circle_id = ? AND account_id IS NOT NULL)`},
}

// FullArchiveData снимает круг целиком. Блобы — описания файлов (без
// содержимого): их таблица отдельная, файлы кладёт сборщик архива.
func (c *Chronicle) FullArchiveData(ctx context.Context, circleID string) (*FullArchiveData, error) {
	tx, err := c.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	out := &FullArchiveData{CircleID: circleID, SnapshotAt: time.Now().UTC()}
	if err := tx.QueryRowContext(ctx, `SELECT name, color FROM circles WHERE id = ?`, circleID).
		Scan(&out.CircleName, &out.Color); err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}
	for _, q := range fullArchiveQueries {
		args := []any{circleID}
		if n := strings.Count(q.query, "?"); n > 1 {
			args = make([]any, n)
			for i := range args {
				args[i] = circleID
			}
		}
		t, err := dumpRows(ctx, tx, q.name, q.query, args...)
		if err != nil {
			return nil, fmt.Errorf("full archive %s: %w", q.name, err)
		}
		out.Tables = append(out.Tables, t)
		if q.name == "posts" {
			out.Posts = len(t.Rows)
		}
	}
	if out.BlobIDs, err = c.circleBlobIDsTx(ctx, tx, circleID); err != nil {
		return nil, err
	}
	blobs, err := dumpBlobs(ctx, tx, out.BlobIDs)
	if err != nil {
		return nil, err
	}
	out.Tables = append(out.Tables, blobs)
	return out, nil
}

// dumpBlobs — описания файлов круга: тип, имя, размер и сумма для проверки
// при оживлении. Владелец и путь на диске — этого сервера, в архив не идут.
func dumpBlobs(ctx context.Context, tx *sql.Tx, ids []string) (FullArchiveTable, error) {
	t := FullArchiveTable{Name: "blobs", Columns: []string{"id", "sha256", "size_bytes", "mime_type", "original_filename", "created_at"}}
	for start := 0; start < len(ids); start += avatarBatch {
		marks, args := inClause(ids[start:min(start+avatarBatch, len(ids))])
		part, err := dumpRows(ctx, tx, "blobs", `SELECT `+strings.Join(t.Columns, ", ")+` FROM blobs WHERE id IN (`+marks+`)`, args...)
		if err != nil {
			return t, err
		}
		t.Rows = append(t.Rows, part.Rows...)
	}
	return t, nil
}

func dumpRows(ctx context.Context, tx *sql.Tx, name, query string, args ...any) (FullArchiveTable, error) {
	t := FullArchiveTable{Name: name}
	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return t, err
	}
	defer rows.Close()
	if t.Columns, err = rows.Columns(); err != nil {
		return t, err
	}
	for rows.Next() {
		vals := make([]any, len(t.Columns))
		ptrs := make([]any, len(vals))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return t, err
		}
		for i, v := range vals {
			// Текст из SQLite приходит и строкой, и байтами — в JSON нужна строка.
			if b, ok := v.([]byte); ok {
				vals[i] = string(b)
			}
		}
		t.Rows = append(t.Rows, vals)
	}
	return t, rows.Err()
}

// FullArchiveFeed — читаемая лента полного архива: все неудалённые записи
// круга, сделанные до before, без отбора по видимости, с комментариями и
// реакциями до того же момента. Лента для глаз; оживляет круг машинная часть.
func (c *Chronicle) FullArchiveFeed(ctx context.Context, circleID string, before time.Time) ([]FeedPost, error) {
	rows, err := c.db.QueryContext(ctx, `
		SELECT id FROM posts
		WHERE circle_id = ? AND deleted = 0 AND created_at < ?
		ORDER BY created_at DESC
	`, circleID, formatTime(before))
	if err != nil {
		return nil, err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// Владелец полного архива видит всё: один отрезок без начала и конца.
	scope := &readScope{spans: []Span{{CanRead: true}}}
	posts, err := c.buildFeedPosts(ctx, ids, circleID, scope)
	if err != nil {
		return nil, err
	}
	for i := range posts {
		posts[i] = filterFeedPostBefore(posts[i], before)
	}
	return posts, nil
}

// FullArchiveDayTitles — нынешние названия всех дней круга, сказанные до before.
func (c *Chronicle) FullArchiveDayTitles(ctx context.Context, circleID string, before time.Time) (map[string]string, error) {
	rows, err := c.db.QueryContext(ctx, `
		SELECT d.entry_date, d.title
		FROM days d
		JOIN day_titles t ON t.event_seq = d.title_event_seq
		WHERE d.circle_id = ? AND d.title IS NOT NULL AND d.title != '' AND t.created_at < ?
	`, circleID, formatTime(before))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]string)
	for rows.Next() {
		var date, title string
		if err := rows.Scan(&date, &title); err != nil {
			return nil, err
		}
		out[date] = title
	}
	return out, rows.Err()
}
