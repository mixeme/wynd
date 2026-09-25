package auth

import (
	"context"
	"fmt"
	"time"
)

func (s *Service) loadInstance(ctx context.Context) (RegistrationMode, string, bool, error) {
	var mode, name string
	var bootstrapped int
	err := s.db.QueryRowContext(ctx, `
		SELECT registration_mode, name, bootstrapped FROM instance_settings WHERE id = 1
	`).Scan(&mode, &name, &bootstrapped)
	if err != nil {
		return "", "", false, fmt.Errorf("load instance: %w", err)
	}
	return RegistrationMode(mode), name, bootstrapped == 1, nil
}

func (s *Service) registrationMode(ctx context.Context) (RegistrationMode, error) {
	mode, _, _, err := s.loadInstance(ctx)
	return mode, err
}

// SetRegistrationMode switches the instance between open, invite and closed registration.
func (s *Service) SetRegistrationMode(ctx context.Context, mode RegistrationMode) error {
	if mode != ModeOpen && mode != ModeInvite && mode != ModeClosed {
		return ErrInvalid
	}
	_, err := s.db.ExecContext(ctx, `
		UPDATE instance_settings SET registration_mode = ?, updated_at = ?
		WHERE id = 1
	`, string(mode), formatTime(time.Now().UTC()))
	return err
}
