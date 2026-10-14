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

// NameErasureAllowed — включён ли у сервера переключатель «Разрешить стирать
// имена и фото» (кадр 9.13, план 46, A3).
func (s *Service) NameErasureAllowed(ctx context.Context) (bool, error) {
	var v int
	if err := s.db.QueryRowContext(ctx, `
		SELECT allow_name_erasure FROM instance_settings WHERE id = 1
	`).Scan(&v); err != nil {
		return false, fmt.Errorf("load name erasure: %w", err)
	}
	return v == 1, nil
}

// SetNameErasure включает и выключает стирание имён. Действует только на
// будущие уходы: тех, кто ушёл раньше, переключатель не касается.
func (s *Service) SetNameErasure(ctx context.Context, allow bool) error {
	v := 0
	if allow {
		v = 1
	}
	_, err := s.db.ExecContext(ctx, `
		UPDATE instance_settings SET allow_name_erasure = ?, updated_at = ? WHERE id = 1
	`, v, formatTime(time.Now().UTC()))
	return err
}
