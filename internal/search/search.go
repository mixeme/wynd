package search

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
)

const defaultLimit = 50
const defaultAuthorLimit = 100

// visibleCarrierSQL — попадание видно, только если видна запись-носитель.
// Раньше видимость проверялась по created_at самой строки FTS, и комментарий
// к невидимой записи находился вместе с миниатюрой этой записи (SRCH-1).
// Для комментария носитель — его запись; день (строка названия) — по
// моменту, когда название дали: новичок того же дня не видит данное до него.
// Комментарий дополнительно должен сам попадать в отрезок: иначе вышедший с
// доступом находил поиском комментарии, написанные после его ухода, которые
// лента скрывает (аудит 2026-09-22).
// Параметр — account_id; запрос обязан делать LEFT JOIN posts p ON p.id = f.post_id.
const visibleCarrierSQL = `
	AND (f.kind = 'day' OR (p.id IS NOT NULL AND p.deleted = 0))
	AND EXISTS (
	  SELECT 1 FROM memberships m
	  JOIN membership_spans ms ON ms.membership_id = m.id
	  WHERE m.circle_id = f.circle_id AND m.account_id = ?
	    AND ms.can_read = 1
	    AND CASE WHEN f.kind = 'day' THEN
	          f.created_at >= ms.started_at
	          AND (ms.ended_at IS NULL OR f.created_at < ms.ended_at)
	        ELSE
	          p.created_at >= ms.started_at
	          AND (ms.ended_at IS NULL OR p.created_at < ms.ended_at)
	          AND (f.kind <> 'comment' OR (
	            f.created_at >= ms.started_at
	            AND (ms.ended_at IS NULL OR f.created_at < ms.ended_at)))
	        END
	)`

// Filters narrows FTS results beyond the text query.
type Filters struct {
	From        string
	To          string
	HasPhoto    bool
	HasLocation bool
	Author      string
}

// Hit is a single FTS match after visibility filtering.
type Hit struct {
	PostID    string `json:"post_id"`
	CommentID string `json:"comment_id,omitempty"`
	// MediaID — вложение, найденное по имени файла или названию звука
	// (Kind file или audio; план 46, C11).
	MediaID string `json:"media_id,omitempty"`
	// MediaBlobID — блоб этого вложения: экран записи находит по нему строку
	// и подсвечивает её.
	MediaBlobID string `json:"media_blob_id,omitempty"`
	CircleID    string `json:"circle_id"`
	AuthorName  string `json:"author_name,omitempty"`
	Kind        string `json:"kind"`
	Title       string `json:"title,omitempty"`
	Snippet     string `json:"snippet"`
	ThumbBlobID string `json:"thumb_blob_id,omitempty"`
	EntryDate   string `json:"entry_date"`
	CreatedAt   string `json:"created_at"`
}

// Service runs FTS5 queries with membership span filtering.
type Service struct {
	ch *chronicle.Chronicle
	db *sql.DB
}

// New creates a search service over the chronicle store.
func New(ch *chronicle.Chronicle) *Service {
	return &Service{ch: ch, db: ch.DB()}
}

// SearchCircle finds matches in one circle; includes author name.
func (s *Service) SearchCircle(ctx context.Context, accountID, circleID, query string, limit int, filters Filters) ([]Hit, error) {
	if strings.TrimSpace(query) == "" {
		return nil, chronicle.ErrInvalid
	}
	if err := s.ch.RequireReader(ctx, circleID, accountID); err != nil {
		return nil, err
	}
	return s.search(ctx, accountID, circleID, query, limit, true, filters)
}

// SearchCircleAuthors lists distinct author names in FTS matches (not circle members).
func (s *Service) SearchCircleAuthors(ctx context.Context, accountID, circleID, query string, filters Filters) ([]string, error) {
	if strings.TrimSpace(query) == "" {
		return nil, chronicle.ErrInvalid
	}
	if err := s.ch.RequireReader(ctx, circleID, accountID); err != nil {
		return nil, err
	}
	filters.Author = ""
	fts, err := ftsQuery(query)
	if err != nil {
		return nil, err
	}
	args := []any{fts, circleID, accountID}
	whereExtra, filterArgs := filterSQL(filters)
	args = append(args, filterArgs...)
	args = append(args, defaultAuthorLimit)

	rows, err := s.db.QueryContext(ctx, fmt.Sprintf(`
		SELECT DISTINCT f.author_name
		FROM content_fts f
		LEFT JOIN posts p ON p.id = f.post_id AND f.post_id != ''
		WHERE content_fts MATCH ?
		  AND f.circle_id = ?
		  AND f.kind != 'day'
		  AND f.author_name != ''%s%s
		ORDER BY f.author_name COLLATE NOCASE
		LIMIT ?
	`, visibleCarrierSQL, whereExtra), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		out = append(out, name)
	}
	return out, rows.Err()
}

// SearchAll finds matches across all circles for the account; includes author name.
func (s *Service) SearchAll(ctx context.Context, accountID, query string, limit int, filters Filters) ([]Hit, error) {
	if strings.TrimSpace(query) == "" {
		return nil, chronicle.ErrInvalid
	}
	return s.search(ctx, accountID, "", query, limit, true, filters)
}

func (s *Service) search(ctx context.Context, accountID, circleID, query string, limit int, withAuthor bool, filters Filters) ([]Hit, error) {
	if limit <= 0 || limit > defaultLimit {
		limit = defaultLimit
	}
	fts, err := ftsQuery(query)
	if err != nil {
		return nil, err
	}
	args := []any{fts, accountID}
	whereCircle := ""
	if circleID != "" {
		whereCircle = " AND f.circle_id = ?"
		args = append(args, circleID)
	}
	whereExtra, filterArgs := filterSQL(filters)
	args = append(args, filterArgs...)
	// Вся фильтрация видимости — в SQL, поэтому запас limit*3 больше не нужен.
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, fmt.Sprintf(`
		SELECT f.post_id, f.comment_id, f.circle_id, f.author_name, f.kind, f.created_at,
			COALESCE(f.entry_date, p.entry_date),
			CASE WHEN f.kind = 'day' THEN f.body ELSE '' END,
			snippet(content_fts, 0, '', '', '…', 32),
			(SELECT pm.blob_id FROM post_media pm
			 WHERE pm.post_id = f.post_id AND pm.kind IN ('photo', 'video')
			 ORDER BY pm.sort_order LIMIT 1),
			CASE WHEN f.kind IN ('file', 'audio')
			  THEN (SELECT pm.blob_id FROM post_media pm WHERE pm.id = f.comment_id) END
		FROM content_fts f
		LEFT JOIN posts p ON p.id = f.post_id AND f.post_id != ''
		WHERE content_fts MATCH ?%s%s%s
		ORDER BY rank
		LIMIT ?
	`, visibleCarrierSQL, whereCircle, whereExtra), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return s.collectHits(ctx, rows, limit, withAuthor)
}

func (s *Service) collectHits(ctx context.Context, rows *sql.Rows, limit int, withAuthor bool) ([]Hit, error) {
	var out []Hit
	for rows.Next() {
		if len(out) >= limit {
			break
		}
		var h Hit
		var commentID string
		var author string
		var created string
		var thumb, mediaBlob sql.NullString
		if err := rows.Scan(&h.PostID, &commentID, &h.CircleID, &author, &h.Kind, &created, &h.EntryDate, &h.Title, &h.Snippet, &thumb, &mediaBlob); err != nil {
			return nil, err
		}
		if h.Kind == "file" || h.Kind == "audio" {
			// В индексе вложения id строки post_media лежит в comment_id.
			h.MediaID = commentID
			h.MediaBlobID = mediaBlob.String
		} else {
			h.CommentID = commentID
		}
		if withAuthor && h.Kind != "day" {
			h.AuthorName = author
		}
		if h.Kind == "day" {
			h.Title = strings.TrimSpace(h.Title)
		}
		h.CreatedAt = created
		if thumb.Valid {
			h.ThumbBlobID = thumb.String
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

const (
	maxQueryBytes  = 256
	maxQueryTokens = 8
)

// ftsQuery собирает запрос FTS из слов: каждое слово в кавычках, слова
// соединяются AND. Раньше весь запрос уходил одной фразой, и «море дача» не
// находило «море и лето на даче». Управляющие символы вырезаются — NUL в
// запросе валил поиск пятисоткой. Каждое слово ищется по началу ("дач"*):
// поиск находит, пока слово набирается, и ловит падежи. Середины слова и
// «точной фразы в кавычках» нет (SRCH-2).
func ftsQuery(q string) (string, error) {
	q = strings.Map(func(r rune) rune {
		if r == '\t' || r == '\n' || r == '\r' {
			return ' '
		}
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, q)
	q = strings.TrimSpace(q)
	if len(q) > maxQueryBytes {
		q = q[:maxQueryBytes]
		// Обрезка могла разрубить руну — отбрасываем хвост до пробела.
		if i := strings.LastIndexByte(q, ' '); i > 0 {
			q = q[:i]
		}
		q = strings.ToValidUTF8(q, "")
	}
	tokens := strings.Fields(q)
	if len(tokens) > maxQueryTokens {
		tokens = tokens[:maxQueryTokens]
	}
	quoted := make([]string, 0, len(tokens))
	for _, t := range tokens {
		t = strings.ReplaceAll(t, `"`, `""`)
		if strings.TrimSpace(t) == "" {
			continue
		}
		quoted = append(quoted, `"`+t+`"*`)
	}
	if len(quoted) == 0 {
		return "", chronicle.ErrInvalid
	}
	return strings.Join(quoted, " AND "), nil
}

func filterSQL(f Filters) (string, []any) {
	var parts []string
	var args []any
	if f.From != "" {
		parts = append(parts, " AND COALESCE(f.entry_date, p.entry_date) >= ?")
		args = append(args, f.From)
	}
	if f.To != "" {
		parts = append(parts, " AND COALESCE(f.entry_date, p.entry_date) <= ?")
		args = append(args, f.To)
	}
	if f.HasPhoto {
		parts = append(parts, ` AND f.post_id != '' AND EXISTS (
			SELECT 1 FROM post_media pm
			JOIN posts pp ON pp.id = pm.post_id AND pp.deleted = 0
			WHERE pm.post_id = f.post_id AND pm.kind IN ('photo', 'video')
		)`)
	}
	if f.HasLocation {
		parts = append(parts, ` AND f.post_id != '' AND EXISTS (
			SELECT 1 FROM post_media pm
			JOIN posts pp ON pp.id = pm.post_id AND pp.deleted = 0
			WHERE pm.post_id = f.post_id AND pm.geo_lat IS NOT NULL AND pm.geo_lng IS NOT NULL
		)`)
	}
	if f.Author != "" {
		parts = append(parts, " AND f.author_name = ? AND f.kind != 'day'")
		args = append(args, f.Author)
	}
	return strings.Join(parts, ""), args
}
