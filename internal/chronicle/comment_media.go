package chronicle

import (
	"context"
	"database/sql"
	"fmt"
)

// MaxCommentMedia — вложений на комментарий (4.28).
const MaxCommentMedia = 10

// normalizeCommentMedia проверяет вложения комментария: фото, голосовое или
// файл. Видео и всё, что принадлежит записи журнала — обложка, кадр, место,
// теги звука, — сюда не идёт.
func normalizeCommentMedia(items []MediaInput) ([]MediaInput, error) {
	if len(items) > MaxCommentMedia {
		return nil, ErrInvalid
	}
	seen := make(map[string]struct{}, len(items))
	out := make([]MediaInput, 0, len(items))
	for _, item := range items {
		if item.BlobID == "" {
			return nil, ErrInvalid
		}
		if item.Kind != MediaPhoto && item.Kind != MediaAttachment {
			return nil, ErrInvalid
		}
		if _, dup := seen[item.BlobID]; dup {
			return nil, ErrInvalid
		}
		seen[item.BlobID] = struct{}{}
		clean := MediaInput{
			BlobID: item.BlobID, Kind: item.Kind,
			Voice: item.Voice, AudioDurationMs: item.AudioDurationMs, AudioPeaks: item.AudioPeaks,
		}
		if err := normalizeVoice(&clean); err != nil {
			return nil, err
		}
		out = append(out, clean)
	}
	return out, nil
}

func (c *Chronicle) insertCommentMedia(ctx context.Context, tx *sql.Tx, commentID string, items []MediaInput) error {
	for i, item := range items {
		id, err := newID()
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO comment_media (id, comment_id, blob_id, kind, sort_order, voice, audio_duration_ms, audio_peaks)
			VALUES (?, ?, ?, ?, ?, ?, NULLIF(?, 0), NULLIF(?, ''))
		`, id, commentID, item.BlobID, string(item.Kind), i,
			boolToInt(item.Voice), item.AudioDurationMs, joinPeaks(item.AudioPeaks)); err != nil {
			return err
		}
	}
	return nil
}

// commentMediaQuerier — *sql.DB и *sql.Tx.
type commentMediaQuerier interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

// listMediaForComments — вложения комментариев по их id, в порядке отправки.
func (c *Chronicle) listMediaForComments(ctx context.Context, q commentMediaQuerier, commentIDs []string) (map[string][]PostMedia, error) {
	out := make(map[string][]PostMedia)
	if len(commentIDs) == 0 {
		return out, nil
	}
	placeholders, args := inClause(commentIDs)
	rows, err := q.QueryContext(ctx, fmt.Sprintf(`
		SELECT cm.comment_id, cm.id, cm.blob_id, cm.kind, cm.sort_order,
			COALESCE(b.original_filename, ''), b.size_bytes, COALESCE(b.mime_type, ''),
			cm.voice, COALESCE(cm.audio_duration_ms, 0), COALESCE(cm.audio_peaks, '')
		FROM comment_media cm
		JOIN blobs b ON b.id = cm.blob_id
		WHERE cm.comment_id IN (%s) ORDER BY cm.comment_id, cm.sort_order
	`, placeholders), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var commentID, peaks string
		var voice int
		var m PostMedia
		if err := rows.Scan(&commentID, &m.ID, &m.BlobID, &m.Kind, &m.SortOrder,
			&m.OriginalFilename, &m.SizeBytes, &m.MimeType,
			&voice, &m.AudioDurationMs, &peaks); err != nil {
			return nil, err
		}
		m.Voice = voice != 0
		m.AudioPeaks = splitPeaks(peaks)
		out[commentID] = append(out[commentID], m)
	}
	return out, rows.Err()
}

// attachCommentMedia дописывает вложения в уже загруженные комментарии.
func (c *Chronicle) attachCommentMedia(ctx context.Context, byPost map[string][]Comment) error {
	var ids []string
	for _, list := range byPost {
		for _, cm := range list {
			ids = append(ids, cm.ID)
		}
	}
	media, err := c.listMediaForComments(ctx, c.db, ids)
	if err != nil || len(media) == 0 {
		return err
	}
	for _, list := range byPost {
		for i := range list {
			list[i].Media = media[list[i].ID]
		}
	}
	return nil
}

// Вложения комментария снимаются явно: сам комментарий стирается пометкой,
// строка остаётся, и каскад по внешнему ключу не срабатывает.

const commentMediaOfComment = `comment_id = ?`
const commentMediaOfPost = `comment_id IN (SELECT id FROM comments WHERE post_id = ?)`

// takeCommentMediaTx удаляет вложения комментариев по условию и возвращает
// их блобы: вызывающий освобождает файлы после коммита.
func (c *Chronicle) takeCommentMediaTx(ctx context.Context, tx *sql.Tx, where, arg string) ([]string, error) {
	rows, err := tx.QueryContext(ctx, `SELECT DISTINCT blob_id FROM comment_media WHERE `+where, arg)
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
	if len(ids) == 0 {
		return nil, nil
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM comment_media WHERE `+where, arg); err != nil {
		return nil, err
	}
	return ids, nil
}

// CommentMediaCount — сколько вложений у комментария; правка текста в пустоту
// разрешена только при них.
func (c *Chronicle) commentMediaCount(ctx context.Context, q querier, commentID string) (int, error) {
	var n int
	err := q.QueryRowContext(ctx, `SELECT COUNT(*) FROM comment_media WHERE comment_id = ?`, commentID).Scan(&n)
	return n, err
}
