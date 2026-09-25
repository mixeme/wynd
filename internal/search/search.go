package search

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
)

const defaultLimit = 50

// Hit is a single FTS match after visibility filtering.
type Hit struct {
	PostID     string `json:"post_id"`
	CommentID  string `json:"comment_id,omitempty"`
	CircleID   string `json:"circle_id"`
	AuthorName string `json:"author_name,omitempty"`
	Kind       string `json:"kind"`
	Title      string `json:"title,omitempty"`
	Snippet    string `json:"snippet"`
	EntryDate  string `json:"entry_date"`
	CreatedAt  string `json:"created_at"`
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
func (s *Service) SearchCircle(ctx context.Context, accountID, circleID, query string, limit int) ([]Hit, error) {
	if strings.TrimSpace(query) == "" {
		return nil, chronicle.ErrInvalid
	}
	if err := s.ch.RequireReader(ctx, circleID, accountID); err != nil {
		return nil, err
	}
	return s.search(ctx, accountID, circleID, query, limit, true)
}

// SearchAll finds matches across all circles for the account; includes author name.
func (s *Service) SearchAll(ctx context.Context, accountID, query string, limit int) ([]Hit, error) {
	if strings.TrimSpace(query) == "" {
		return nil, chronicle.ErrInvalid
	}
	return s.search(ctx, accountID, "", query, limit, true)
}

func (s *Service) search(ctx context.Context, accountID, circleID, query string, limit int, withAuthor bool) ([]Hit, error) {
	if limit <= 0 || limit > defaultLimit {
		limit = defaultLimit
	}
	ftsQuery := ftsEscape(query)
	args := []any{ftsQuery, accountID}
	whereCircle := ""
	if circleID != "" {
		whereCircle = " AND f.circle_id = ?"
		args = append(args, circleID)
	}
	args = append(args, limit*3)

	rows, err := s.db.QueryContext(ctx, fmt.Sprintf(`
		SELECT f.post_id, f.comment_id, f.circle_id, f.author_name, f.kind, f.created_at,
			COALESCE(f.entry_date, p.entry_date),
			CASE WHEN f.kind = 'day' THEN f.body ELSE '' END,
			snippet(content_fts, 0, '', '', '…', 32)
		FROM content_fts f
		LEFT JOIN posts p ON p.id = f.post_id AND f.post_id != ''
		WHERE content_fts MATCH ?
		  AND EXISTS (
		    SELECT 1 FROM memberships m
		    JOIN membership_spans ms ON ms.membership_id = m.id
		    WHERE m.circle_id = f.circle_id AND m.account_id = ?
		      AND ms.can_read = 1
		      AND f.created_at >= ms.started_at
		      AND (ms.ended_at IS NULL OR f.created_at < ms.ended_at)
		  )%s
		ORDER BY rank
		LIMIT ?
	`, whereCircle), args...)
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
		if err := rows.Scan(&h.PostID, &commentID, &h.CircleID, &author, &h.Kind, &created, &h.EntryDate, &h.Title, &h.Snippet); err != nil {
			return nil, err
		}
		h.CommentID = commentID
		if withAuthor && h.Kind != "day" {
			h.AuthorName = author
		}
		if h.Kind == "day" {
			h.Title = strings.TrimSpace(h.Title)
		}
		h.CreatedAt = created
		out = append(out, h)
	}
	return out, rows.Err()
}

func ftsEscape(q string) string {
	q = strings.TrimSpace(q)
	q = strings.ReplaceAll(q, `"`, `""`)
	return `"` + q + `"`
}
