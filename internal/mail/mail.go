package mail

import (
	"context"
	"crypto/tls"
	"database/sql"
	"fmt"
	"log"
	"net"
	"net/smtp"
	"strings"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/auth"
	"gitea.mixdep.ru/mix/wynd/internal/store"
)

// Config is instance SMTP settings.
type Config struct {
	Host       string     `json:"host"`
	Port       int        `json:"port"`
	Username   string     `json:"username"`
	Password   string     `json:"password"`
	From       string     `json:"from"`
	TestSentAt *time.Time `json:"test_sent_at,omitempty"`
}

// Service sends email via SMTP or falls back to logging on loopback.
type Service struct {
	db       *sql.DB
	loopback bool
	fallback auth.CodeDelivery
	dial     func(ctx context.Context, addr string) (net.Conn, error)
}

// New wraps a migrated SQLite store.
func New(st store.Store, loopback bool, fallback auth.CodeDelivery) (*Service, error) {
	s, ok := st.(*store.SQLite)
	if !ok {
		return nil, fmt.Errorf("mail: store must be *store.SQLite")
	}
	db := s.DB()
	if db == nil {
		return nil, fmt.Errorf("mail: closed store")
	}
	if fallback == nil {
		fallback = auth.LogCodes{Logger: log.Default()}
	}
	return &Service{
		db:       db,
		loopback: loopback,
		fallback: fallback,
		dial:     defaultDial,
	}, nil
}

// DB exposes the underlying connection for tests.
func (s *Service) DB() *sql.DB {
	return s.db
}

// LoadConfig reads SMTP settings from instance_settings.
func (s *Service) LoadConfig(ctx context.Context) (Config, error) {
	var cfg Config
	var testSent sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT smtp_host, smtp_port, smtp_username, smtp_password, smtp_from, smtp_test_sent_at
		FROM instance_settings WHERE id = 1
	`).Scan(&cfg.Host, &cfg.Port, &cfg.Username, &cfg.Password, &cfg.From, &testSent)
	if err != nil {
		return Config{}, fmt.Errorf("mail: load config: %w", err)
	}
	if testSent.Valid && testSent.String != "" {
		t, err := time.Parse(time.RFC3339Nano, testSent.String)
		if err != nil {
			return Config{}, fmt.Errorf("mail: parse test_sent_at: %w", err)
		}
		cfg.TestSentAt = &t
	}
	return cfg, nil
}

// SaveConfig persists SMTP settings (test_sent_at is unchanged).
func (s *Service) SaveConfig(ctx context.Context, cfg Config) error {
	if strings.TrimSpace(cfg.Host) == "" || strings.TrimSpace(cfg.From) == "" {
		return ErrInvalid
	}
	if cfg.Port <= 0 {
		cfg.Port = 587
	}
	_, err := s.db.ExecContext(ctx, `
		UPDATE instance_settings
		SET smtp_host = ?, smtp_port = ?, smtp_username = ?, smtp_password = ?, smtp_from = ?
		WHERE id = 1
	`, strings.TrimSpace(cfg.Host), cfg.Port, cfg.Username, cfg.Password, strings.TrimSpace(cfg.From))
	if err != nil {
		return fmt.Errorf("mail: save config: %w", err)
	}
	return nil
}

// Configured reports whether SMTP is ready to send.
func (s *Service) Configured(ctx context.Context) (bool, error) {
	cfg, err := s.LoadConfig(ctx)
	if err != nil {
		return false, err
	}
	return configured(cfg), nil
}

func configured(cfg Config) bool {
	return strings.TrimSpace(cfg.Host) != "" && strings.TrimSpace(cfg.From) != "" && cfg.Port > 0
}

// SendCode implements auth.CodeDelivery.
func (s *Service) SendCode(ctx context.Context, email, code string) error {
	email = strings.TrimSpace(email)
	if email == "" || len(code) != 6 {
		return ErrInvalid
	}
	cfg, err := s.LoadConfig(ctx)
	if err != nil {
		return err
	}
	if !configured(cfg) {
		if s.loopback {
			return s.fallback.SendCode(ctx, email, code)
		}
		return ErrNotConfigured
	}
	subject := "Код входа Wynd"
	body := fmt.Sprintf("Ваш код входа: %s\n\nКод действует 15 минут.", code)
	return s.sendMessage(ctx, cfg, email, subject, body)
}

// SendTest sends a verification email and records smtp_test_sent_at.
func (s *Service) SendTest(ctx context.Context, to string) error {
	to = strings.TrimSpace(to)
	if to == "" {
		return ErrInvalid
	}
	cfg, err := s.LoadConfig(ctx)
	if err != nil {
		return err
	}
	if !configured(cfg) {
		return ErrNotConfigured
	}
	subject := "Проверка почты Wynd"
	body := "Это проверочное письмо от сервера Wynd."
	if err := s.sendMessage(ctx, cfg, to, subject, body); err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = s.db.ExecContext(ctx, `
		UPDATE instance_settings SET smtp_test_sent_at = ? WHERE id = 1
	`, now)
	if err != nil {
		return fmt.Errorf("mail: record test sent: %w", err)
	}
	return nil
}

// SendPlain sends a plain-text email when SMTP is configured.
func (s *Service) SendPlain(ctx context.Context, to, subject, body string) error {
	to = strings.TrimSpace(to)
	if to == "" || strings.TrimSpace(subject) == "" {
		return ErrInvalid
	}
	cfg, err := s.LoadConfig(ctx)
	if err != nil {
		return err
	}
	if !configured(cfg) {
		return ErrNotConfigured
	}
	return s.sendMessage(ctx, cfg, to, subject, body)
}

func (s *Service) sendMessage(ctx context.Context, cfg Config, to, subject, body string) error {
	msg := buildMessage(cfg.From, to, subject, body)
	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	conn, err := s.dial(ctx, addr)
	if err != nil {
		return fmt.Errorf("mail: dial %s: %w", addr, err)
	}
	defer conn.Close()

	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}

	client, err := smtp.NewClient(conn, cfg.Host)
	if err != nil {
		return fmt.Errorf("mail: smtp client: %w", err)
	}
	defer client.Close()

	if ok, _ := client.Extension("STARTTLS"); ok {
		if err := client.StartTLS(&tls.Config{ServerName: cfg.Host}); err != nil {
			return fmt.Errorf("mail: starttls: %w", err)
		}
	}

	if cfg.Username != "" {
		auth := smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("mail: auth: %w", err)
		}
	}

	if err := client.Mail(cfg.From); err != nil {
		return fmt.Errorf("mail: mail from: %w", err)
	}
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("mail: rcpt: %w", err)
	}
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("mail: data: %w", err)
	}
	if _, err := w.Write(msg); err != nil {
		return fmt.Errorf("mail: write: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("mail: data close: %w", err)
	}
	return client.Quit()
}

func buildMessage(from, to, subject, body string) []byte {
	var b strings.Builder
	b.WriteString("From: ")
	b.WriteString(from)
	b.WriteString("\r\nTo: ")
	b.WriteString(to)
	b.WriteString("\r\nSubject: ")
	b.WriteString(subject)
	b.WriteString("\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n")
	b.WriteString(body)
	if !strings.HasSuffix(body, "\n") {
		b.WriteString("\r\n")
	}
	return []byte(b.String())
}

func defaultDial(_ context.Context, addr string) (net.Conn, error) {
	return net.Dial("tcp", addr)
}
