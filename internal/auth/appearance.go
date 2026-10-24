package auth

import (
	"context"
	"database/sql"
	"errors"
)

// Форма аватаров (план 46, A16): одна на все устройства человека — хранится
// у учётной записи, устройство применяет её у себя и пишет сюда при смене.
const (
	AvatarCircle = "circle"
	AvatarSquare = "square"
)

// AvatarShape — форма аватаров учётной записи; нет записи — круг.
func (s *Service) AvatarShape(ctx context.Context, accountID string) (string, error) {
	var shape string
	err := s.db.QueryRowContext(ctx, `SELECT avatar_shape FROM accounts WHERE id = ?`, accountID).Scan(&shape)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	if shape != AvatarSquare {
		shape = AvatarCircle
	}
	return shape, nil
}

// SetAvatarShape — «Аватары» в «Настройки → Приложение».
func (s *Service) SetAvatarShape(ctx context.Context, accountID, shape string) error {
	if shape != AvatarCircle && shape != AvatarSquare {
		return ErrInvalid
	}
	res, err := s.db.ExecContext(ctx, `UPDATE accounts SET avatar_shape = ? WHERE id = ?`, shape, accountID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}
