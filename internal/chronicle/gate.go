package chronicle

import (
	"context"
	"database/sql"
	"time"
)

// Шлюз прав круга. Все проверки членства живут здесь: раньше их было
// сорок вызовов c.membership(...) по всему пакету, и каждый сам решал,
// смотреть ли на статус, — отсюда и правка от исключённого (SEC-3).
//
// Порядок строгости: membership (есть ли членство) → requireReader
// (может ли читать) → requireWriter (может ли писать сейчас) →
// requireAuthor (своё ли это сказанное). Права на круг — RequireOwner,
// RequireSettings, RequireCanInvite.

func (c *Chronicle) membership(ctx context.Context, q querier, circleID, accountID string) (Membership, error) {
	var m Membership
	var canSettings int
	var status string
	var created, updated string
	err := q.QueryRowContext(ctx, `
		SELECT id, circle_id, account_id, identity_id, can_settings, status, created_at, updated_at
		FROM memberships WHERE circle_id = ? AND account_id = ?
	`, circleID, accountID).Scan(&m.ID, &m.CircleID, &m.AccountID, &m.IdentityID, &canSettings, &status, &created, &updated)
	if err == sql.ErrNoRows {
		return Membership{}, ErrNotFound
	}
	if err != nil {
		return Membership{}, err
	}
	m.CanSettings = canSettings == 1
	m.Status = MembershipStatus(status)
	m.CreatedAt, _ = parseTime(created)
	m.UpdatedAt, _ = parseTime(updated)
	return m, nil
}

func (c *Chronicle) requireReader(ctx context.Context, circleID, accountID string) error {
	mem, err := c.membership(ctx, c.db, circleID, accountID)
	if err != nil {
		return err
	}
	if mem.Status == StatusGone {
		return ErrForbidden
	}
	spans, err := c.visibilitySpans(ctx, circleID, accountID)
	if err != nil {
		return err
	}
	for _, sp := range spans {
		if sp.CanRead {
			return nil
		}
	}
	return ErrForbidden
}

func (c *Chronicle) requireWriter(ctx context.Context, circleID, accountID string, now time.Time) (Membership, error) {
	mem, err := c.membership(ctx, c.db, circleID, accountID)
	if err != nil {
		return Membership{}, err
	}
	canWrite, err := c.CanWrite(ctx, circleID, accountID, now)
	if err != nil {
		return Membership{}, err
	}
	if !canWrite {
		return Membership{}, ErrForbidden
	}
	return mem, nil
}

// requireAuthor gates editing and deleting one's own content: the actor must be
// the author *and* be able to write right now. Исключённый и вышедший с
// доступом получают forbidden (план 42, раздел B «Право писать»).
func (c *Chronicle) requireAuthor(ctx context.Context, q querier, circleID, accountID, identityID string, now time.Time) (Membership, error) {
	mem, err := c.membership(ctx, q, circleID, accountID)
	if err != nil {
		return Membership{}, err
	}
	if mem.IdentityID != identityID {
		return Membership{}, ErrForbidden
	}
	canWrite, err := c.CanWrite(ctx, circleID, accountID, now)
	if err != nil {
		return Membership{}, err
	}
	if !canWrite {
		return Membership{}, ErrForbidden
	}
	return mem, nil
}

// requireSettingsTx — право менять настройки круга: владелец или участник с
// can_settings. Читает в переданной транзакции.
func (c *Chronicle) requireSettingsTx(ctx context.Context, tx *sql.Tx, circleID, accountID string) (Membership, error) {
	mem, err := c.membership(ctx, tx, circleID, accountID)
	if err != nil {
		return Membership{}, err
	}
	if mem.Status != StatusActive {
		return Membership{}, ErrForbidden
	}
	if mem.CanSettings {
		return mem, nil
	}
	owner, err := c.circleOwner(ctx, tx, circleID)
	if err != nil {
		return Membership{}, err
	}
	if owner != accountID {
		return Membership{}, ErrForbidden
	}
	return mem, nil
}

// RequireOwner returns ErrForbidden unless accountID owns the circle.
func (c *Chronicle) RequireOwner(ctx context.Context, circleID, accountID string) error {
	owner, err := c.circleOwner(ctx, c.db, circleID)
	if err != nil {
		return err
	}
	if owner != accountID {
		return ErrForbidden
	}
	return nil
}

// RequireSettings returns ErrForbidden unless the account is an active member
// with can_settings (the owner always has it).
func (c *Chronicle) RequireSettings(ctx context.Context, circleID, accountID string) error {
	mem, err := c.membership(ctx, c.db, circleID, accountID)
	if err != nil {
		return err
	}
	if mem.Status != StatusActive || !mem.CanSettings {
		return ErrForbidden
	}
	return nil
}

// RequireCanInvite returns ErrForbidden unless the account may create invites.
func (c *Chronicle) RequireCanInvite(ctx context.Context, circleID, accountID string) error {
	settings, err := c.GetInviteSettings(ctx, circleID)
	if err != nil {
		return err
	}
	if settings.InviteWho == "owner" {
		return c.RequireOwner(ctx, circleID, accountID)
	}
	mem, err := c.membership(ctx, c.db, circleID, accountID)
	if err != nil {
		return err
	}
	if mem.Status != StatusActive {
		return ErrForbidden
	}
	return nil
}
