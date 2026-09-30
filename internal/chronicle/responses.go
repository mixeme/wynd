package chronicle

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// «Отклики» (3.12–3.13): комментарии, реакции, названия и обложки дней —
// всё, что люди делают с уже лежащими записями, в порядке журнала.
//
// Источник — живые строки сказанного, а не лог событий: удалённый
// комментарий, снятая реакция и схлопнутый день исчезают сами, без
// отдельной чистки. Правки записей сюда не входят (wynd.html, «Правила
// ленты»): это продолжение сказанного автором, а не отклик на него. Своих
// действий нет — что сделал сам, человек знает.

// Виды откликов.
const (
	ResponseComment  = "comment"
	ResponseReaction = "reaction"
	ResponseDayTitle = "day_title"
	ResponseDayCover = "day_cover"
)

// ResponsePageLimit — сколько откликов отдаёт одна страница.
const ResponsePageLimit = 100

// Response — один отклик.
type Response struct {
	Kind              string
	Seq               int64
	At                time.Time
	ActorIdentityID   string
	ActorName         string
	ActorAvatarBlobID string
	CommentID         string
	Body              string
	Emoji             string
	Title             string
	PostID            string
	EntryDate         string
	CoverBlobID       string
}

// ResponsePostRef — запись, к которой относится отклик: рамка на 3.13.
type ResponsePostRef struct {
	ID               string
	AuthorIdentityID string
	AuthorName       string
	EntryDate        string
	CreatedAt        time.Time
	Excerpt          string
	CoverBlobID      string
}

// ResponsesPage — страница откликов и ссылки на записи и дни.
type ResponsesPage struct {
	Items    []Response
	Posts    map[string]ResponsePostRef
	DayPosts map[string]int
	ReadSeq  int64
	HasMore  bool
}

// responsesSQL собирает отклики круга, видимые участнику и не его. where —
// условие на seq (`seq < ?` для страницы, `seq > ?` для счётчика).
//
// Видимость — как у ленты: отклик должен попасть в отрезок чтения, и запись,
// к которой он относится, — тоже. День видим, если видно само событие.
func responsesSQL(selectList, where string) string {
	return fmt.Sprintf(`
		SELECT %s FROM (
			SELECT '%s' AS kind, c.event_seq AS seq, c.created_at AS at,
				c.identity_id AS actor, c.author_name AS actor_name,
				c.id AS comment_id, COALESCE(c.body, '') AS body, '' AS emoji, '' AS title,
				c.post_id AS post_id, '' AS entry_date, '' AS cover_blob_id
			FROM comments c
			JOIN posts p ON p.id = c.post_id AND p.deleted = 0
			WHERE c.circle_id = ? AND c.deleted = 0 AND c.identity_id != ?
			  AND %s AND %s
			UNION ALL
			SELECT '%s', r.event_seq, r.created_at, r.identity_id, r.author_name,
				'', '', r.emoji, '', r.post_id, '', ''
			FROM reactions r
			JOIN posts p ON p.id = r.post_id AND p.deleted = 0
			WHERE r.circle_id = ? AND r.deleted = 0 AND r.emoji != '' AND r.identity_id != ?
			  AND %s AND %s
			UNION ALL
			SELECT '%s', d.title_event_seq, e.created_at, e.actor_identity_id, e.actor_name,
				'', '', '', d.title, '', d.entry_date, ''
			FROM days d
			JOIN events e ON e.seq = d.title_event_seq
			WHERE d.circle_id = ? AND d.title IS NOT NULL AND d.title != ''
			  AND COALESCE(e.actor_identity_id, '') != ?
			  AND %s
			UNION ALL
			SELECT '%s', d.cover_event_seq, e.created_at, e.actor_identity_id, e.actor_name,
				'', '', '', '', COALESCE(d.cover_post_id, ''), d.entry_date, COALESCE(d.cover_blob_id, '')
			FROM days d
			JOIN events e ON e.seq = d.cover_event_seq
			WHERE d.circle_id = ? AND d.cover_blob_id IS NOT NULL
			  AND COALESCE(e.actor_identity_id, '') != ?
			  AND %s
		) WHERE %s`,
		selectList,
		ResponseComment, sqlVisibleAt("c.created_at"), sqlVisibleAt("p.created_at"),
		ResponseReaction, sqlVisibleAt("r.created_at"), sqlVisibleAt("p.created_at"),
		ResponseDayTitle, sqlVisibleAt("e.created_at"),
		ResponseDayCover, sqlVisibleAt("e.created_at"),
		where)
}

func responsesArgs(circleID, accountID, identityID string) []any {
	vis := []any{circleID, accountID}
	var args []any
	// комментарии и реакции: круг, не я, отклик виден, запись видна
	for range 2 {
		args = append(args, circleID, identityID)
		args = append(args, vis...)
		args = append(args, vis...)
	}
	// название и обложка дня: круг, не я, событие видно
	for range 2 {
		args = append(args, circleID, identityID)
		args = append(args, vis...)
	}
	return args
}

// CircleHasOthers — был ли в круге кто-то ещё, хоть раз. В круге из одного
// (3.7) откликаться некому, и вкладки «Отклики» нет.
func (c *Chronicle) CircleHasOthers(ctx context.Context, circleID, accountID string) (bool, error) {
	var n int
	err := c.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM memberships WHERE circle_id = ? AND account_id != ?
	`, circleID, accountID).Scan(&n)
	return n > 0, err
}

// ResponseReadSeq — докуда участник видел отклики.
func (c *Chronicle) ResponseReadSeq(ctx context.Context, circleID, accountID string) (int64, error) {
	var seq int64
	err := c.db.QueryRowContext(ctx, `
		SELECT seq FROM response_cursors WHERE account_id = ? AND circle_id = ?
	`, accountID, circleID).Scan(&seq)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	return seq, err
}

// SetResponseReadSeq двигает отметку вперёд; назад не отступает.
func (c *Chronicle) SetResponseReadSeq(ctx context.Context, circleID, accountID string, seq int64, now time.Time) error {
	if seq < 0 {
		return ErrInvalid
	}
	if err := c.requireReader(ctx, circleID, accountID); err != nil {
		return err
	}
	_, err := c.db.ExecContext(ctx, `
		INSERT INTO response_cursors (account_id, circle_id, seq, updated_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(account_id, circle_id) DO UPDATE SET
			seq = CASE WHEN excluded.seq > response_cursors.seq THEN excluded.seq ELSE response_cursors.seq END,
			updated_at = excluded.updated_at
	`, accountID, circleID, seq, formatTime(utcOrNow(now)))
	return err
}

// UnreadResponses — сколько откликов новее отметки: число на вкладке и
// точка на улочке.
func (c *Chronicle) UnreadResponses(ctx context.Context, circleID, accountID string) (int, error) {
	mem, err := c.membership(ctx, c.db, circleID, accountID)
	if err != nil {
		return 0, err
	}
	readSeq, err := c.ResponseReadSeq(ctx, circleID, accountID)
	if err != nil {
		return 0, err
	}
	args := append(responsesArgs(circleID, accountID, mem.IdentityID), readSeq)
	var n int
	err = c.db.QueryRowContext(ctx, responsesSQL("COUNT(*)", "seq > ?"), args...).Scan(&n)
	return n, err
}

// Responses отдаёт страницу откликов от свежего к старому. before = 0 —
// с самого свежего.
func (c *Chronicle) Responses(ctx context.Context, circleID, accountID string, before int64) (ResponsesPage, error) {
	if err := c.requireReader(ctx, circleID, accountID); err != nil {
		return ResponsesPage{}, err
	}
	mem, err := c.membership(ctx, c.db, circleID, accountID)
	if err != nil {
		return ResponsesPage{}, err
	}
	readSeq, err := c.ResponseReadSeq(ctx, circleID, accountID)
	if err != nil {
		return ResponsesPage{}, err
	}
	where := "1 = 1"
	args := responsesArgs(circleID, accountID, mem.IdentityID)
	if before > 0 {
		where = "seq < ?"
		args = append(args, before)
	}
	args = append(args, ResponsePageLimit+1)
	rows, err := c.db.QueryContext(ctx, responsesSQL(
		"kind, seq, at, actor, actor_name, comment_id, body, emoji, title, post_id, entry_date, cover_blob_id",
		where)+" ORDER BY seq DESC LIMIT ?", args...)
	if err != nil {
		return ResponsesPage{}, err
	}
	defer rows.Close()

	page := ResponsesPage{ReadSeq: readSeq, Posts: map[string]ResponsePostRef{}, DayPosts: map[string]int{}}
	for rows.Next() {
		var r Response
		var at string
		var actor sql.NullString
		if err := rows.Scan(&r.Kind, &r.Seq, &at, &actor, &r.ActorName, &r.CommentID, &r.Body,
			&r.Emoji, &r.Title, &r.PostID, &r.EntryDate, &r.CoverBlobID); err != nil {
			return ResponsesPage{}, err
		}
		r.ActorIdentityID = actor.String
		if r.At, err = parseTime(at); err != nil {
			return ResponsesPage{}, fmt.Errorf("response at: %w", err)
		}
		page.Items = append(page.Items, r)
	}
	if err := rows.Err(); err != nil {
		return ResponsesPage{}, err
	}
	if len(page.Items) > ResponsePageLimit {
		page.Items = page.Items[:ResponsePageLimit]
		page.HasMore = true
	}
	if err := c.attachResponseRefs(ctx, circleID, accountID, &page); err != nil {
		return ResponsesPage{}, err
	}
	return page, nil
}

func (c *Chronicle) attachResponseRefs(ctx context.Context, circleID, accountID string, page *ResponsesPage) error {
	var postIDs, actors []string
	seenPost := map[string]bool{}
	seenActor := map[string]bool{}
	for _, r := range page.Items {
		if r.PostID != "" && !seenPost[r.PostID] && r.Kind != ResponseDayCover {
			seenPost[r.PostID] = true
			postIDs = append(postIDs, r.PostID)
		}
		if r.ActorIdentityID != "" && !seenActor[r.ActorIdentityID] {
			seenActor[r.ActorIdentityID] = true
			actors = append(actors, r.ActorIdentityID)
		}
	}
	avatars, err := c.IdentityAvatarBlobIDs(ctx, actors)
	if err != nil {
		return err
	}
	for i := range page.Items {
		page.Items[i].ActorAvatarBlobID = avatars[page.Items[i].ActorIdentityID]
	}
	if len(postIDs) > 0 {
		posts, err := c.loadPostsByIDs(ctx, postIDs)
		if err != nil {
			return err
		}
		media, err := c.listMediaForPosts(ctx, postIDs)
		if err != nil {
			return err
		}
		for id, p := range posts {
			ref := ResponsePostRef{
				ID: id, AuthorIdentityID: p.IdentityID, AuthorName: p.AuthorName,
				EntryDate: p.EntryDate, CreatedAt: p.CreatedAt, Excerpt: excerpt(p.Body),
			}
			ref.CoverBlobID = responseCover(media[id])
			page.Posts[id] = ref
		}
	}
	hasDay := false
	for _, r := range page.Items {
		if r.Kind == ResponseDayTitle || r.Kind == ResponseDayCover {
			hasDay = true
			break
		}
	}
	if hasDay {
		counts, err := c.visiblePostCountsByDay(ctx, circleID, accountID)
		if err != nil {
			return err
		}
		for _, r := range page.Items {
			if r.EntryDate != "" {
				page.DayPosts[r.EntryDate] = counts[r.EntryDate]
			}
		}
	}
	return nil
}

// responseCover — обложка записи для рамки: выбранная автором, иначе первый
// снимок или ролик. Вложения обложкой не бывают.
func responseCover(items []PostMedia) string {
	first := ""
	for _, m := range items {
		if m.Kind != MediaPhoto && m.Kind != MediaVideo {
			continue
		}
		if m.IsCover {
			return m.BlobID
		}
		if first == "" {
			first = m.BlobID
		}
	}
	return first
}

// excerpt — первая строка записи для рамки; целиком её рисует клиент с
// многоточием, сервер только не шлёт лишнего.
func excerpt(body string) string {
	line := strings.TrimSpace(body)
	if i := strings.IndexByte(line, '\n'); i >= 0 {
		line = strings.TrimSpace(line[:i])
	}
	runes := []rune(line)
	if len(runes) > 140 {
		line = string(runes[:140])
	}
	return line
}
