package chronicle

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"
)

func (c *Chronicle) visiblePostIDs(ctx context.Context, circleID, accountID, extraWhere, orderBy string, limit int, extraArgs ...any) ([]string, error) {
	query := fmt.Sprintf(`
		SELECT id FROM posts
		WHERE circle_id = ? AND deleted = 0
		  AND %s
		  AND %s
		ORDER BY %s
		LIMIT ?
	`, extraWhere, sqlVisibleAt("posts.created_at"), orderBy)
	args := append([]any{circleID}, extraArgs...)
	// Берём на одну запись больше потолка: превышение видно и попадает в лог,
	// а не остаётся молчаливым обрезанием (CHR-3).
	args = append(args, circleID, accountID, limit+1)
	rows, err := c.db.QueryContext(ctx, query, args...)
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
	if len(ids) > limit {
		log.Printf("chronicle: circle %s превысил потолок снимка (%d записей): показаны последние %d",
			circleID, limit, limit)
		ids = ids[:limit]
	}
	return ids, nil
}

func (c *Chronicle) buildFeedPosts(ctx context.Context, postIDs []string, circleID string, scope *readScope) ([]FeedPost, error) {
	if len(postIDs) == 0 {
		return nil, nil
	}
	posts, err := c.loadPostsByIDs(ctx, postIDs)
	if err != nil {
		return nil, err
	}
	mediaByPost, err := c.listMediaForPosts(ctx, postIDs)
	if err != nil {
		return nil, err
	}
	commentsByPost, err := c.listCommentsForPosts(ctx, postIDs, scope)
	if err != nil {
		return nil, err
	}
	reactionsByPost, err := c.listReactionsForPosts(ctx, postIDs, scope)
	if err != nil {
		return nil, err
	}
	out := make([]FeedPost, 0, len(postIDs))
	for _, id := range postIDs {
		post, ok := posts[id]
		if !ok || post.Deleted || post.CircleID != circleID {
			continue
		}
		if !scope.canRead(post.CreatedAt) {
			continue
		}
		out = append(out, FeedPost{
			Post:      post,
			Media:     mediaByPost[id],
			Comments:  commentsByPost[id],
			Reactions: reactionsByPost[id],
		})
	}
	return out, nil
}

func (c *Chronicle) loadPostsByIDs(ctx context.Context, ids []string) (map[string]Post, error) {
	placeholders, args := inClause(ids)
	rows, err := c.db.QueryContext(ctx, fmt.Sprintf(`
		SELECT id, circle_id, event_seq, identity_id, author_name, body,
			entry_date, captured_at, created_at, edit_window_sec, editable_until, deleted
		FROM posts WHERE id IN (%s)
	`, placeholders), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]Post, len(ids))
	for rows.Next() {
		var p Post
		var body, captured sql.NullString
		var ew sql.NullInt64
		var until sql.NullString
		var deleted int
		var created string
		if err := rows.Scan(&p.ID, &p.CircleID, &p.EventSeq, &p.IdentityID, &p.AuthorName, &body,
			&p.EntryDate, &captured, &created, &ew, &until, &deleted); err != nil {
			return nil, err
		}
		if body.Valid {
			p.Body = body.String
		}
		if captured.Valid {
			t, _ := parseTime(captured.String)
			p.CapturedAt = &t
		}
		p.CreatedAt, _ = parseTime(created)
		p.EditWindow, _ = editWindowFromSQL(ew)
		p.EditableUntil, _ = parseEditableUntil(until)
		p.Deleted = deleted == 1
		out[p.ID] = p
	}
	return out, rows.Err()
}

func (c *Chronicle) listMediaForPosts(ctx context.Context, postIDs []string) (map[string][]PostMedia, error) {
	placeholders, args := inClause(postIDs)
	rows, err := c.db.QueryContext(ctx, fmt.Sprintf(postMediaSelect+`
		FROM post_media pm
		JOIN blobs b ON b.id = pm.blob_id
		WHERE pm.post_id IN (%s) ORDER BY pm.post_id, pm.sort_order
	`, placeholders), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string][]PostMedia)
	for rows.Next() {
		m, err := scanPostMedia(rows)
		if err != nil {
			return nil, err
		}
		out[m.PostID] = append(out[m.PostID], m)
	}
	return out, rows.Err()
}

func (c *Chronicle) listCommentsForPosts(ctx context.Context, postIDs []string, scope *readScope) (map[string][]Comment, error) {
	placeholders, args := inClause(postIDs)
	rows, err := c.db.QueryContext(ctx, fmt.Sprintf(`
		SELECT id, circle_id, post_id, event_seq, identity_id, author_name, body,
			created_at, edit_window_sec, editable_until, deleted
		FROM comments WHERE post_id IN (%s) AND deleted = 0 ORDER BY post_id, created_at
	`, placeholders), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanCommentsMap(rows, scope)
}

func (c *Chronicle) listReactionsForPosts(ctx context.Context, postIDs []string, scope *readScope) (map[string][]Reaction, error) {
	placeholders, args := inClause(postIDs)
	rows, err := c.db.QueryContext(ctx, fmt.Sprintf(`
		SELECT id, circle_id, post_id, event_seq, identity_id, author_name, emoji,
			created_at, edit_window_sec, editable_until, deleted
		FROM reactions WHERE post_id IN (%s) AND deleted = 0 ORDER BY post_id, created_at
	`, placeholders), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanReactionsMap(rows, scope)
}

func scanCommentsMap(rows *sql.Rows, scope *readScope) (map[string][]Comment, error) {
	out := make(map[string][]Comment)
	for rows.Next() {
		cm, err := scanCommentRow(rows)
		if err != nil {
			return nil, err
		}
		if scope.canRead(cm.CreatedAt) {
			out[cm.PostID] = append(out[cm.PostID], cm)
		}
	}
	return out, rows.Err()
}

func scanReactionsMap(rows *sql.Rows, scope *readScope) (map[string][]Reaction, error) {
	out := make(map[string][]Reaction)
	for rows.Next() {
		rx, err := scanReactionRow(rows)
		if err != nil {
			return nil, err
		}
		if scope.canRead(rx.CreatedAt) {
			out[rx.PostID] = append(out[rx.PostID], rx)
		}
	}
	return out, rows.Err()
}

func inClause(ids []string) (string, []any) {
	parts := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		parts[i] = "?"
		args[i] = id
	}
	return strings.Join(parts, ","), args
}
