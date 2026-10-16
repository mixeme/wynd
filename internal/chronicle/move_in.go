package chronicle

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// Перенос круга по ссылке (план 53), новый сервер: кто вставил ссылку (он и
// станет владельцем), карточка круга с прежнего сервера, ход и итог. Ссылку
// сервер помнит вместе с просьбой о месте (7.22).

// Состояния переноса на новом сервере.
const (
	IncomingDraft    = "draft"
	IncomingPulling  = "pulling"
	IncomingReviving = "reviving"
	IncomingDone     = "done"
	IncomingFailed   = "failed"
)

// IncomingMove — перенос, каким его видит новый сервер.
type IncomingMove struct {
	ID          string
	AccountID   string
	Link        string
	SourceURL   string
	SourceName  string
	CircleName  string
	CircleColor string
	Posts       int
	Members     int
	Bytes       int64
	FrozenAt    *time.Time
	ExpiresAt   time.Time
	SendInvites bool
	Status      string
	PulledBytes int64
	CircleID    string
	ResultPosts int
	ResultNames int
	// ResultBytes — файлы поднятого круга: их сверяет прежний сервер.
	ResultBytes int64
	// ResultTextBytes — текст поднятого круга сейчас (C42): к итогу 7.25 для
	// экрана, чтобы круг без файлов не показывал 0 Б.
	ResultTextBytes int64
	Verified    bool
	Invited     int
	NotJoined   int
	Error       string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// IncomingCard — карточка круга с прежнего сервера (7.20).
type IncomingCard struct {
	SourceURL   string
	SourceName  string
	CircleName  string
	CircleColor string
	Posts       int
	Members     int
	Bytes       int64
	FrozenAt    *time.Time
	ExpiresAt   time.Time
}

const incomingCols = `id, account_id, link, source_url, source_name, circle_name, circle_color,
	posts, members, bytes, frozen_at, expires_at, send_invites, status, pulled_bytes,
	COALESCE(circle_id, ''), COALESCE(result_posts, 0), COALESCE(result_names, 0), COALESCE(result_bytes, 0),
	COALESCE((SELECT text_bytes FROM circles WHERE circles.id = incoming_moves.circle_id), 0),
	verified, invited, not_joined, COALESCE(error, ''), created_at, updated_at`

func scanIncoming(row interface{ Scan(...any) error }) (*IncomingMove, error) {
	var m IncomingMove
	var frozen sql.NullString
	var expires, created, updated string
	if err := row.Scan(&m.ID, &m.AccountID, &m.Link, &m.SourceURL, &m.SourceName, &m.CircleName, &m.CircleColor,
		&m.Posts, &m.Members, &m.Bytes, &frozen, &expires, &m.SendInvites, &m.Status, &m.PulledBytes,
		&m.CircleID, &m.ResultPosts, &m.ResultNames, &m.ResultBytes, &m.ResultTextBytes,
		&m.Verified, &m.Invited, &m.NotJoined, &m.Error, &created, &updated); err != nil {
		return nil, err
	}
	m.FrozenAt = optTime(frozen)
	m.ExpiresAt = parseTimeOrZero(expires)
	m.CreatedAt = parseTimeOrZero(created)
	m.UpdatedAt = parseTimeOrZero(updated)
	return &m, nil
}

// Active — сервер забирает или поднимает круг.
func (m *IncomingMove) Active() bool {
	return m.Status == IncomingPulling || m.Status == IncomingReviving
}

// PutIncomingDraft — вставили ссылку (7.20): прежний черновик этой учётки
// заменяется новым (7.24: «новую ссылку вставьте в это же поле»). Пока
// сервер забирает другой круг этой учётки — ErrMoveBusy.
func (c *Chronicle) PutIncomingDraft(ctx context.Context, accountID, link string, card IncomingCard, now time.Time) (*IncomingMove, error) {
	id, err := newID()
	if err != nil {
		return nil, err
	}
	now = utcOrNow(now)
	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var busy int
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM incoming_moves WHERE account_id = ? AND status IN ('pulling', 'reviving')
	`, accountID).Scan(&busy); err != nil {
		return nil, err
	}
	if busy > 0 {
		return nil, ErrMoveBusy
	}
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM incoming_moves WHERE account_id = ? AND status IN ('draft', 'failed')
	`, accountID); err != nil {
		return nil, err
	}
	var frozen any
	if card.FrozenAt != nil {
		frozen = formatTime(*card.FrozenAt)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO incoming_moves (id, account_id, link, source_url, source_name, circle_name, circle_color,
			posts, members, bytes, frozen_at, expires_at, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'draft', ?, ?)
	`, id, accountID, link, card.SourceURL, card.SourceName, card.CircleName, card.CircleColor,
		card.Posts, card.Members, card.Bytes, frozen, formatTime(card.ExpiresAt), formatTime(now), formatTime(now)); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return c.GetIncoming(ctx, id)
}

// GetIncoming — входящий перенос по id.
func (c *Chronicle) GetIncoming(ctx context.Context, id string) (*IncomingMove, error) {
	m, err := scanIncoming(c.db.QueryRowContext(ctx, `SELECT `+incomingCols+` FROM incoming_moves WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return m, err
}

// LatestIncoming — последний входящий перенос учётки; nil — не было.
func (c *Chronicle) LatestIncoming(ctx context.Context, accountID string) (*IncomingMove, error) {
	m, err := scanIncoming(c.db.QueryRowContext(ctx, `
		SELECT `+incomingCols+` FROM incoming_moves WHERE account_id = ?
		ORDER BY created_at DESC, rowid DESC LIMIT 1
	`, accountID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return m, err
}

// StartIncoming — «Поднять круг здесь» (7.20): черновик или неудавшийся
// перенос — в работу. false — он уже идёт или закончен.
func (c *Chronicle) StartIncoming(ctx context.Context, id string, sendInvites bool, now time.Time) (bool, error) {
	res, err := c.db.ExecContext(ctx, `
		UPDATE incoming_moves SET status = 'pulling', send_invites = ?, error = NULL, updated_at = ?
		WHERE id = ? AND status IN ('draft', 'failed')
	`, sendInvites, formatTime(utcOrNow(now)), id)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// SetIncomingPulled — сколько байт архива уже здесь («3,1 из 8,4 ГБ»).
func (c *Chronicle) SetIncomingPulled(ctx context.Context, id string, pulled int64) error {
	_, err := c.db.ExecContext(ctx, `
		UPDATE incoming_moves SET pulled_bytes = ?, updated_at = ? WHERE id = ?
	`, pulled, formatTime(time.Now().UTC()), id)
	return err
}

// SetIncomingStatus — забирает, поднимает или сбой (с кодом причины).
func (c *Chronicle) SetIncomingStatus(ctx context.Context, id, status, errCode string) error {
	var e any
	if errCode != "" {
		e = errCode
	}
	_, err := c.db.ExecContext(ctx, `
		UPDATE incoming_moves SET status = ?, error = ?, updated_at = ? WHERE id = ?
	`, status, e, formatTime(time.Now().UTC()), id)
	return err
}

// SetIncomingRevived — круг поднят (7.25, 7.26): итог записан сразу, до
// «принято» прежнему серверу, — перезапуск между ними не поднимет круг второй
// раз.
func (c *Chronicle) SetIncomingRevived(ctx context.Context, id, circleID string, posts, names int, bytes int64, verified bool, invited, notJoined int) error {
	_, err := c.db.ExecContext(ctx, `
		UPDATE incoming_moves SET circle_id = ?, result_posts = ?, result_names = ?,
			result_bytes = ?, verified = ?, invited = ?, not_joined = ?, updated_at = ?
		WHERE id = ?
	`, circleID, posts, names, bytes, verified, invited, notJoined, formatTime(time.Now().UTC()), id)
	return err
}

// FinishIncoming — прежний сервер знает, что круг принят: перенос закончен,
// ключ ссылки больше не нужен — стирается.
func (c *Chronicle) FinishIncoming(ctx context.Context, id string) error {
	_, err := c.db.ExecContext(ctx, `
		UPDATE incoming_moves SET status = 'done', link = '', error = NULL, updated_at = ? WHERE id = ?
	`, formatTime(time.Now().UTC()), id)
	return err
}

// ActiveIncoming — переносы, прерванные остановкой сервера: при запуске
// работа подхватывает их.
func (c *Chronicle) ActiveIncoming(ctx context.Context) ([]string, error) {
	rows, err := c.db.QueryContext(ctx, `SELECT id FROM incoming_moves WHERE status IN ('pulling', 'reviving')`)
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
	return ids, rows.Err()
}
