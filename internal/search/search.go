package search

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

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

// SearchAll finds matches across all circles for the account; omits author name.
func (s *Service) SearchAll(ctx context.Context, accountID, query string, limit int) ([]Hit, error) {
	if strings.TrimSpace(query) == "" {
		return nil, chronicle.ErrInvalid
	}
	return s.searchAll(ctx, accountID, query, limit)
}

func (s *Service) searchAll(ctx context.Context, accountID, query string, limit int) ([]Hit, error) {
	if limit <= 0 || limit > defaultLimit {
		limit = defaultLimit
	}
	ftsQuery := ftsEscape(query)
	rows, err := s.db.QueryContext(ctx, `
		SELECT f.post_id, f.comment_id, f.circle_id, f.author_name, f.kind, f.created_at,
			snippet(content_fts, 0, '', '', '…', 32)
		FROM content_fts f
		WHERE content_fts MATCH ?
		  AND EXISTS (
		    SELECT 1 FROM memberships m
		    JOIN membership_spans ms ON ms.membership_id = m.id
		    WHERE m.circle_id = f.circle_id AND m.account_id = ?
		      AND ms.can_read = 1
		      AND f.created_at >= ms.started_at
		      AND (ms.ended_at IS NULL OR f.created_at < ms.ended_at)
		  )
		ORDER BY rank
		LIMIT ?
	`, ftsQuery, accountID, limit*3)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return s.collectHits(ctx, rows, accountID, limit, false)
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
			snippet(content_fts, 0, '', '', '…', 32)
		FROM content_fts f
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
	return s.collectHits(ctx, rows, accountID, limit, withAuthor)
}

func (s *Service) collectHits(ctx context.Context, rows *sql.Rows, accountID string, limit int, withAuthor bool) ([]Hit, error) {
	var out []Hit
	for rows.Next() {
		if len(out) >= limit {
			break
		}
		var h Hit
		var commentID string
		var author string
		var created string
		if err := rows.Scan(&h.PostID, &commentID, &h.CircleID, &author, &h.Kind, &created, &h.Snippet); err != nil {
			return nil, err
		}
		h.CommentID = commentID
		if withAuthor {
			h.AuthorName = author
		}
		h.CreatedAt = created
		t, err := parseTime(created)
		if err != nil {
			continue
		}
		ok, err := s.ch.CanReadEvent(ctx, h.CircleID, accountID, t)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		entryDate, err := s.entryDateForHit(ctx, h)
		if err != nil {
			return nil, err
		}
		h.EntryDate = entryDate
		out = append(out, h)
	}
	return out, rows.Err()
}

func (s *Service) entryDateForHit(ctx context.Context, h Hit) (string, error) {
	var entryDate string
	err := s.db.QueryRowContext(ctx, `SELECT entry_date FROM posts WHERE id = ?`, h.PostID).Scan(&entryDate)
	if err == sql.ErrNoRows {
		return "", chronicle.ErrNotFound
	}
	return entryDate, err
}

func ftsEscape(q string) string {
	q = strings.TrimSpace(q)
	q = strings.ReplaceAll(q, `"`, `""`)
	return `"` + q + `"`
}

func parseTime(s string) (time.Time, error) {
	return time.Parse(time.RFC3339Nano, s)
}
