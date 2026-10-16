package chronicle

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// Перенос круга по ссылке (план 53, A1 этап 3), прежний сервер: владелец
// выдаёт ссылку — круг замирает; новый сервер приходит по ней, забирает полный
// архив без шифрования и сообщает «принято и сверено»; владелец завершает
// перенос (круг удаляется) или отменяет его (круг оживает).

// Состояния переноса на прежнем сервере.
const (
	MoveWaiting   = "waiting"
	MovePulling   = "pulling"
	MoveAccepted  = "accepted"
	MoveCanceled  = "canceled"
	MoveCompleted = "completed"
)

// MoveLinkTTL — сколько живёт ссылка переноса (6.34: «живёт 72 часа»).
const MoveLinkTTL = 72 * time.Hour

var (
	// ErrMoveBusy — у круга уже идёт перенос или он замер под архив с удалением.
	ErrMoveBusy = errors.New("chronicle: circle move already in progress")
	// ErrMoveTaken — по ссылке уже пришёл другой сервер (ссылка одноразовая).
	ErrMoveTaken = errors.New("chronicle: circle move taken by another server")
	// ErrMoveExpired — ссылка переноса больше не действует.
	ErrMoveExpired = errors.New("chronicle: circle move link expired")
)

// CircleMove — перенос круга, каким его видит прежний сервер.
type CircleMove struct {
	ID            string
	CircleID      string
	Token         string
	StartedBy     string
	StartedAt     time.Time
	ExpiresAt     time.Time
	Status        string
	DestURL       string
	DestName      string
	ArchiveStatus string
	ArchiveBytes  int64
	PulledBytes   int64
	// Что в архиве: записей, имён, байт файлов — для сверки (6.35).
	ArchivePosts      int
	ArchiveNames      int
	ArchiveMediaBytes int64
	AcceptedAt        *time.Time
	AcceptedPosts     int
	AcceptedNames     int
	AcceptedBytes     int64
	EndedAt           *time.Time
}

// Live — перенос не закончен: круг стоит замершим ради него.
func (m *CircleMove) Live() bool {
	return m.Status == MoveWaiting || m.Status == MovePulling || m.Status == MoveAccepted
}

// LinkAlive — по ссылке ещё можно забирать круг.
func (m *CircleMove) LinkAlive(now time.Time) bool {
	return (m.Status == MoveWaiting || m.Status == MovePulling) && now.Before(m.ExpiresAt)
}

const circleMoveCols = `id, circle_id, token, started_by_account_id, started_at, expires_at, status,
	COALESCE(dest_url, ''), COALESCE(dest_name, ''), archive_status, archive_bytes, pulled_bytes,
	archive_posts, archive_names, archive_media_bytes,
	accepted_at, COALESCE(accepted_posts, 0), COALESCE(accepted_names, 0), COALESCE(accepted_bytes, 0), ended_at`

func scanCircleMove(row interface{ Scan(...any) error }) (*CircleMove, error) {
	var m CircleMove
	var started, expires string
	var accepted, ended sql.NullString
	if err := row.Scan(&m.ID, &m.CircleID, &m.Token, &m.StartedBy, &started, &expires, &m.Status,
		&m.DestURL, &m.DestName, &m.ArchiveStatus, &m.ArchiveBytes, &m.PulledBytes,
		&m.ArchivePosts, &m.ArchiveNames, &m.ArchiveMediaBytes,
		&accepted, &m.AcceptedPosts, &m.AcceptedNames, &m.AcceptedBytes, &ended); err != nil {
		return nil, err
	}
	m.StartedAt = parseTimeOrZero(started)
	m.ExpiresAt = parseTimeOrZero(expires)
	m.AcceptedAt = optTime(accepted)
	m.EndedAt = optTime(ended)
	return &m, nil
}

// LiveMove — незаконченный перенос круга; nil — переноса нет.
func (c *Chronicle) LiveMove(ctx context.Context, circleID string) (*CircleMove, error) {
	m, err := scanCircleMove(c.db.QueryRowContext(ctx, `
		SELECT `+circleMoveCols+` FROM circle_moves
		WHERE circle_id = ? AND status IN ('waiting', 'pulling', 'accepted')
		ORDER BY started_at DESC, rowid DESC LIMIT 1
	`, circleID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return m, err
}

// MoveByTokenHash — перенос по ключу ссылки (обмен серверов).
func (c *Chronicle) MoveByTokenHash(ctx context.Context, hash string) (*CircleMove, error) {
	m, err := scanCircleMove(c.db.QueryRowContext(ctx, `
		SELECT `+circleMoveCols+` FROM circle_moves WHERE token_hash = ?
	`, hash))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return m, err
}

// GetMove — перенос по id.
func (c *Chronicle) GetMove(ctx context.Context, id string) (*CircleMove, error) {
	m, err := scanCircleMove(c.db.QueryRowContext(ctx, `
		SELECT `+circleMoveCols+` FROM circle_moves WHERE id = ?
	`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return m, err
}

// StartMove — «Остановить круг и получить ссылку» (6.33): круг замирает той же
// транзакцией, в ленте — «Кот начал перенос круга» (3.14). Круг, который уже
// замер (под архив с удалением или под другой перенос), — ErrMoveBusy.
func (c *Chronicle) StartMove(ctx context.Context, circleID, accountID, token, tokenHash string, now time.Time) (*CircleMove, error) {
	id, err := newID()
	if err != nil {
		return nil, err
	}
	now = utcOrNow(now)
	actor, err := c.membership(ctx, c.db, circleID, accountID)
	if err != nil {
		return nil, err
	}
	actorName, err := c.identityName(ctx, c.db, actor.IdentityID)
	if err != nil {
		return nil, err
	}
	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, `
		UPDATE circles SET frozen_at = ?, updated_at = ? WHERE id = ? AND frozen_at IS NULL
	`, formatTime(now), formatTime(now), circleID)
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, ErrMoveBusy
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO circle_moves (id, circle_id, started_by_account_id, started_at, token, token_hash, expires_at, status)
		VALUES (?, ?, ?, ?, ?, ?, ?, 'waiting')
	`, id, circleID, accountID, formatTime(now), token, tokenHash, formatTime(now.Add(MoveLinkTTL))); err != nil {
		return nil, err
	}
	if _, err := c.appendEvent(ctx, tx, appendEventInput{
		circleID: circleID, eventType: "circle_move_started", isService: true,
		actorIdentityID: actor.IdentityID, actorName: actorName,
		summary: summaryCircleMoveStarted(c.identityGender(ctx, tx, actor.IdentityID), actorName),
		now:     now,
	}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return c.GetMove(ctx, id)
}

// ClaimMove — новый сервер пришёл по ссылке. Первый закрепляет перенос за
// собой; тот же адрес — продолжение после обрыва; другой — ErrMoveTaken.
// started — архив под перенос ещё не собирали: его пора собирать.
func (c *Chronicle) ClaimMove(ctx context.Context, id, destURL, destName string, now time.Time) (started bool, err error) {
	now = utcOrNow(now)
	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	m, err := scanCircleMove(tx.QueryRowContext(ctx, `SELECT `+circleMoveCols+` FROM circle_moves WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return false, ErrNotFound
	}
	if err != nil {
		return false, err
	}
	if !m.LinkAlive(now) {
		return false, ErrMoveExpired
	}
	if m.DestURL != "" && m.DestURL != destURL {
		return false, ErrMoveTaken
	}
	started = m.ArchiveStatus == "none" || m.ArchiveStatus == "failed"
	archive := m.ArchiveStatus
	if started {
		archive = "building"
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE circle_moves SET status = 'pulling', dest_url = ?, dest_name = ?, archive_status = ?
		WHERE id = ?
	`, destURL, destName, archive, id); err != nil {
		return false, err
	}
	return started, tx.Commit()
}

// SetMoveArchive — что с архивом под перенос: сбой или файл стёрт ("none").
func (c *Chronicle) SetMoveArchive(ctx context.Context, id, status string) error {
	_, err := c.db.ExecContext(ctx, `
		UPDATE circle_moves SET archive_status = ?, archive_bytes = 0 WHERE id = ?
	`, status, id)
	return err
}

// MoveArchiveReady — архив под перенос собран: длина файла и что в нём.
func (c *Chronicle) MoveArchiveReady(ctx context.Context, id string, size int64, posts, names int, mediaBytes int64) error {
	_, err := c.db.ExecContext(ctx, `
		UPDATE circle_moves SET archive_status = 'ready', archive_bytes = ?,
			archive_posts = ?, archive_names = ?, archive_media_bytes = ?
		WHERE id = ? AND archive_status = 'building'
	`, size, posts, names, mediaBytes, id)
	return err
}

// SetMovePulled — сколько байт архива новый сервер уже забрал (6.34: «3,1 из
// 8,4 ГБ»). Только вперёд: повтор куска после обрыва ход не откатывает.
func (c *Chronicle) SetMovePulled(ctx context.Context, id string, upTo int64) error {
	_, err := c.db.ExecContext(ctx, `
		UPDATE circle_moves SET pulled_bytes = MAX(pulled_bytes, ?) WHERE id = ?
	`, upTo, id)
	return err
}

// AcceptMove — новый сервер принял круг и сверил (6.35). В ленте — «Кот
// перенёс круг на wynd.u-slavy.ru» (3.15), от имени начавшего перенос.
func (c *Chronicle) AcceptMove(ctx context.Context, id string, posts, names int, bytes int64, destHost string, now time.Time) error {
	now = utcOrNow(now)
	m, err := c.GetMove(ctx, id)
	if err != nil {
		return err
	}
	if m.Status == MoveAccepted {
		return nil
	}
	if m.Status != MovePulling {
		return ErrMoveExpired
	}
	actor, err := c.membership(ctx, c.db, m.CircleID, m.StartedBy)
	if err != nil {
		return err
	}
	actorName, err := c.identityName(ctx, c.db, actor.IdentityID)
	if err != nil {
		return err
	}
	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, `
		UPDATE circle_moves SET status = 'accepted', accepted_at = ?, accepted_posts = ?,
			accepted_names = ?, accepted_bytes = ?, pulled_bytes = archive_bytes
		WHERE id = ? AND status = 'pulling'
	`, formatTime(now), posts, names, bytes, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return tx.Commit()
	}
	if _, err := c.appendEvent(ctx, tx, appendEventInput{
		circleID: m.CircleID, eventType: "circle_moved", isService: true,
		actorIdentityID: actor.IdentityID, actorName: actorName,
		summary: summaryCircleMoved(c.identityGender(ctx, tx, actor.IdentityID), actorName, destHost),
		now:     now,
	}); err != nil {
		return err
	}
	return tx.Commit()
}

// CancelMove — «Отменить перенос» (6.34, 6.35): ключ гаснет, круг оживает.
func (c *Chronicle) CancelMove(ctx context.Context, circleID, accountID string, now time.Time) (*CircleMove, error) {
	now = utcOrNow(now)
	m, err := c.LiveMove(ctx, circleID)
	if err != nil {
		return nil, err
	}
	if m == nil {
		return nil, ErrNotFound
	}
	if _, err := c.db.ExecContext(ctx, `
		UPDATE circle_moves SET status = 'canceled', ended_at = ? WHERE id = ?
	`, formatTime(now), m.ID); err != nil {
		return nil, err
	}
	if err := c.UnfreezeCircle(ctx, circleID, accountID, now); err != nil {
		return nil, err
	}
	return m, nil
}

// EndedMoveArchives — переносы, чей архив больше не нужен: ссылка истекла, не
// дойдя до «принято», или перенос закончен. Файлы их архивов стираются.
func (c *Chronicle) EndedMoveArchives(ctx context.Context, now time.Time) ([]string, error) {
	rows, err := c.db.QueryContext(ctx, `
		SELECT id FROM circle_moves
		WHERE archive_status IN ('building', 'ready')
		  AND (status IN ('accepted', 'canceled', 'completed')
		       OR (status IN ('waiting', 'pulling') AND expires_at <= ?))
	`, formatTime(utcOrNow(now)))
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

// RecoverMoveArchives — при старте: недособранные архивы переносов — сбой;
// keep — те, чей готовый файл ещё нужен, остальное в каталоге переносов
// стирается.
func (c *Chronicle) RecoverMoveArchives(ctx context.Context, now time.Time) (keep []string, err error) {
	if _, err := c.db.ExecContext(ctx, `
		UPDATE circle_moves SET archive_status = 'failed', archive_bytes = 0 WHERE archive_status = 'building'
	`); err != nil {
		return nil, err
	}
	rows, err := c.db.QueryContext(ctx, `
		SELECT id FROM circle_moves
		WHERE archive_status = 'ready' AND status IN ('waiting', 'pulling') AND expires_at > ?
	`, formatTime(utcOrNow(now)))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		keep = append(keep, id)
	}
	return keep, rows.Err()
}
