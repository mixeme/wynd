package auth

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
	"gitea.mixdep.ru/mix/wynd/internal/store"
	"gitea.mixdep.ru/mix/wynd/internal/version"
)

const (
	codeTTL           = 15 * time.Minute
	codeMaxAttempts   = 3
	sessionTTL        = 30 * 24 * time.Hour
	adminSessionTTL   = 8 * time.Hour
	ipRateLimit       = 10
	ipRateLimitWindow = time.Hour
	emailRateLimit    = 5
)

// CodeDelivery sends login codes. On loopback without SMTP codes are logged.
type CodeDelivery interface {
	SendCode(ctx context.Context, email, code string) error
}

// LogCodes prints codes to the standard logger and optionally appends to File.
type LogCodes struct {
	Logger *log.Logger
	File   string
}

func (l LogCodes) SendCode(_ context.Context, email, code string) error {
	logger := l.Logger
	if logger == nil {
		logger = log.Default()
	}
	line := fmt.Sprintf("auth code for %s: %s", email, code)
	logger.Print(line)
	if l.File == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(l.File), 0o700); err != nil {
		return fmt.Errorf("auth code log dir: %w", err)
	}
	f, err := os.OpenFile(l.File, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("auth code log file: %w", err)
	}
	defer func() { _ = f.Close() }()
	if _, err := fmt.Fprintf(f, "%s %s\n", time.Now().UTC().Format(time.RFC3339), line); err != nil {
		return fmt.Errorf("auth code log write: %w", err)
	}
	return nil
}

// Service handles accounts, invites, codes and sessions.
type Service struct {
	db        *sql.DB
	chronicle *chronicle.Chronicle
	mailer    CodeDelivery
	version   string
	loopback  bool

	loginLimiter *failLimiter
	// Ввод кода: свой потолок неудач на адрес. Без него знающий чужую почту
	// сжигал три попытки её кода сколько угодно раз (аудит 2026-09-22, SEC-8).
	verifyLimiter *failLimiter
}

// New wraps a migrated SQLite store.
func New(st store.Store, ch *chronicle.Chronicle, mailer CodeDelivery, loopback bool) (*Service, error) {
	s, ok := st.(*store.SQLite)
	if !ok {
		return nil, fmt.Errorf("auth: store must be *store.SQLite")
	}
	db := s.DB()
	if db == nil {
		return nil, fmt.Errorf("auth: closed store")
	}
	if mailer == nil {
		mailer = LogCodes{}
	}
	return &Service{
		db:        db,
		chronicle: ch,
		mailer:    mailer,
		version:   version.String(),
		loopback:  loopback,

		loginLimiter:  newFailLimiter(),
		verifyLimiter: newFailLimiterWith(verifyFailWindow, verifyMaxFails, verifyLockout),
	}, nil
}

// SetLoopback updates whether this process treats the instance as loopback.
func (s *Service) SetLoopback(v bool) {
	s.loopback = v
}

// DB exposes the underlying connection for tests.
func (s *Service) DB() *sql.DB {
	return s.db
}

func (s *Service) Instance(ctx context.Context) (InstanceInfo, error) {
	mode, name, bootstrapped, err := s.loadInstance(ctx)
	if err != nil {
		return InstanceInfo{}, err
	}
	return InstanceInfo{
		Name:             name,
		Version:          s.version,
		RegistrationMode: mode,
		Loopback:         s.loopback,
		Bootstrapped:     bootstrapped,
	}, nil
}
