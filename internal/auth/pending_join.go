package auth

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
)

// CompleteCircleJoinInput finishes a deferred invite join after verify.
type CompleteCircleJoinInput struct {
	AccountID string
	CircleID  string
	Name      string
	Now       time.Time
}

func (s *Service) insertPendingCircleJoin(ctx context.Context, tx *sql.Tx, accountID, circleID string, when time.Time) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO pending_circle_joins (account_id, circle_id, created_at)
		VALUES (?, ?, ?)
		ON CONFLICT(account_id, circle_id) DO UPDATE SET created_at = excluded.created_at
	`, accountID, circleID, formatTime(when))
	if err != nil {
		return fmt.Errorf("insert pending circle join: %w", err)
	}
	return nil
}

func (s *Service) hasPendingCircleJoin(ctx context.Context, accountID, circleID string) (bool, error) {
	var id string
	err := s.db.QueryRowContext(ctx, `
		SELECT account_id FROM pending_circle_joins WHERE account_id = ? AND circle_id = ?
	`, accountID, circleID).Scan(&id)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (s *Service) deletePendingCircleJoin(ctx context.Context, tx *sql.Tx, accountID, circleID string) error {
	_, err := tx.ExecContext(ctx, `
		DELETE FROM pending_circle_joins WHERE account_id = ? AND circle_id = ?
	`, accountID, circleID)
	return err
}

// CompleteCircleJoin adds the account to the circle when verify deferred naming.
func (s *Service) CompleteCircleJoin(ctx context.Context, in CompleteCircleJoinInput) error {
	if in.AccountID == "" || in.CircleID == "" || in.Name == "" {
		return ErrInvalid
	}
	when := in.Now.UTC()
	if when.IsZero() {
		when = time.Now().UTC()
	}
	ok, err := s.hasPendingCircleJoin(ctx, in.AccountID, in.CircleID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrForbidden
	}
	if s.chronicle == nil {
		return fmt.Errorf("chronicle required for circle join")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, _, err := s.chronicle.JoinInTx(ctx, tx, chronicle.JoinInput{
		CircleID:  in.CircleID,
		AccountID: in.AccountID,
		Name:      in.Name,
		Now:       when,
	}); err != nil {
		return err
	}
	if err := s.deletePendingCircleJoin(ctx, tx, in.AccountID, in.CircleID); err != nil {
		return err
	}
	return tx.Commit()
}
