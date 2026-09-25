package mail

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"database/sql"
	"fmt"
	"log"
	"mime"
	"net"
	netmail "net/mail"
	"net/smtp"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/auth"
	"gitea.mixdep.ru/mix/wynd/internal/store"
	"gitea.mixdep.ru/mix/wynd/internal/xtime"
)

// Config is instance SMTP settings.
type Config struct {
	Host       string     `json:"host"`
	Port       int        `json:"port"`
	Username   string     `json:"username"`
	Password   string     `json:"password"`
	From       string     `json:"from"`
	TestSentAt *time.Time `json:"test_sent_at,omitempty"`
	LastError  string     `json:"last_error,omitempty"`
}

const defaultSendTimeout = 15 * time.Second

// sendTimeout bounds one SMTP conversation. Tests may shorten it.
var sendTimeout = defaultSendTimeout

// Service sends email via SMTP or falls back to logging on loopback.
type Service struct {
	db *sql.DB
	// loopback меняется из обработчика панели, а читается при каждой
	// отправке: обычное поле здесь было гонкой данных (QLT-4).
	loopback atomic.Bool
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
	svc := &Service{
		db:       db,
		fallback: fallback,
		dial:     defaultDial,
	}
	svc.loopback.Store(loopback)
	return svc, nil
}

// SetLoopback updates whether this process treats the instance as loopback.
func (s *Service) SetLoopback(v bool) {
	s.loopback.Store(v)
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
		SELECT smtp_host, smtp_port, smtp_username, smtp_password, smtp_from, smtp_test_sent_at, smtp_last_error
		FROM instance_settings WHERE id = 1
	`).Scan(&cfg.Host, &cfg.Port, &cfg.Username, &cfg.Password, &cfg.From, &testSent, &cfg.LastError)
	if err != nil {
		return Config{}, fmt.Errorf("mail: load config: %w", err)
	}
	if testSent.Valid && testSent.String != "" {
		t, err := xtime.Parse(testSent.String)
		if err != nil {
			return Config{}, fmt.Errorf("mail: parse test_sent_at: %w", err)
		}
		cfg.TestSentAt = &t
	}
	return cfg, nil
}

// execer is *sql.DB or *sql.Tx: SaveConfig и SaveConfigTx пишут одинаково.
type execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// SaveConfig persists SMTP settings (test_sent_at is unchanged).
func (s *Service) SaveConfig(ctx context.Context, cfg Config) error {
	return saveConfig(ctx, s.db, cfg)
}

// SaveConfigTx persists SMTP settings inside a caller's transaction, so the
// relay is stored together with the rest of a multi-step change (bootstrap).
func (s *Service) SaveConfigTx(ctx context.Context, tx *sql.Tx, cfg Config) error {
	return saveConfig(ctx, tx, cfg)
}

func saveConfig(ctx context.Context, db execer, cfg Config) error {
	if strings.TrimSpace(cfg.Host) == "" || strings.TrimSpace(cfg.From) == "" {
		return ErrInvalid
	}
	if cfg.Port <= 0 {
		cfg.Port = 587
	}
	_, err := db.ExecContext(ctx, `
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
		if s.loopback.Load() {
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
	err = s.sendMessage(ctx, cfg, to, subject, body)
	if recErr := s.recordSMTPTest(ctx, err); recErr != nil && err == nil {
		return recErr
	}
	return err
}

func (s *Service) recordSMTPTest(ctx context.Context, sendErr error) error {
	now := xtime.Format(time.Now())
	errText := ""
	if sendErr != nil {
		errText = sendErr.Error()
	}
	_, err := s.db.ExecContext(ctx, `
		UPDATE instance_settings SET smtp_test_sent_at = ?, smtp_last_error = ? WHERE id = 1
	`, now, errText)
	if err != nil {
		return fmt.Errorf("mail: record test: %w", err)
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

// Probe dials the relay, upgrades TLS, authenticates, and quits.
// It does not send a message and does not persist settings.
func (s *Service) Probe(ctx context.Context, cfg Config) error {
	if strings.TrimSpace(cfg.Host) == "" {
		return ErrNotConfigured
	}
	ctx, cancel := context.WithTimeout(ctx, sendTimeout)
	defer cancel()
	client, err := s.smtpClient(ctx, cfg)
	if err != nil {
		return err
	}
	defer client.Close()
	if err := client.Quit(); err != nil {
		return sendErr(ReasonProtocol, err)
	}
	return nil
}

func (s *Service) sendMessage(ctx context.Context, cfg Config, to, subject, body string) error {
	ctx, cancel := context.WithTimeout(ctx, sendTimeout)
	defer cancel()

	from := envelopeAddress(cfg.From)
	rcpt := envelopeAddress(to)
	if from == "" || rcpt == "" {
		return ErrInvalid
	}

	msg := buildMessage(cfg.From, to, subject, body)
	client, err := s.smtpClient(ctx, cfg)
	if err != nil {
		return err
	}
	defer client.Close()

	if err := client.Mail(from); err != nil {
		return sendErr(ReasonProtocol, err)
	}
	if err := client.Rcpt(rcpt); err != nil {
		return sendErr(ReasonProtocol, err)
	}
	w, err := client.Data()
	if err != nil {
		return sendErr(ReasonProtocol, err)
	}
	if _, err := w.Write(msg); err != nil {
		return sendErr(ReasonProtocol, err)
	}
	if err := w.Close(); err != nil {
		return sendErr(ReasonProtocol, err)
	}
	if err := client.Quit(); err != nil {
		return sendErr(ReasonProtocol, err)
	}
	return nil
}

func (s *Service) smtpClient(ctx context.Context, cfg Config) (*smtp.Client, error) {
	host := strings.TrimSpace(cfg.Host)
	port := cfg.Port
	if port <= 0 {
		port = 587
	}
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	conn, err := s.dial(ctx, addr)
	if err != nil {
		return nil, sendErrf(ReasonDial, "dial %s: %w", addr, err)
	}
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}
	if useImplicitTLS(port) {
		tlsConn := tls.Client(conn, &tls.Config{ServerName: host})
		if err := tlsConn.HandshakeContext(ctx); err != nil {
			_ = conn.Close()
			return nil, sendErrf(ReasonTLS, "tls: %w", err)
		}
		conn = tlsConn
	}
	client, err := smtp.NewClient(conn, host)
	if err != nil {
		_ = conn.Close()
		return nil, sendErrf(ReasonProtocol, "smtp client: %w", err)
	}
	if !useImplicitTLS(port) {
		starttls, _ := client.Extension("STARTTLS")
		switch {
		case starttls:
			if err := client.StartTLS(&tls.Config{ServerName: host}); err != nil {
				_ = client.Close()
				return nil, sendErrf(ReasonTLS, "starttls: %w", err)
			}
		case isLoopbackHost(host):
			// Локальный релей на той же машине — открытый канал допустим.
		default:
			// Посредник может вырезать 250-STARTTLS из ответа, и письмо с
			// кодом входа уйдёт открытым текстом. Молча так не делаем (SEC-5).
			_ = client.Close()
			return nil, sendErrf(ReasonSTARTTLSRequired, "STARTTLS required by %s", addr)
		}
	}
	if err := authenticate(client, cfg); err != nil {
		_ = client.Close()
		return nil, err
	}
	return client, nil
}

func authenticate(client *smtp.Client, cfg Config) error {
	if strings.TrimSpace(cfg.Username) == "" {
		return nil
	}
	ok, mechs := client.Extension("AUTH")
	if !ok {
		return sendErrf(ReasonAuth, "server has no AUTH")
	}
	var a smtp.Auth
	upper := strings.ToUpper(mechs)
	switch {
	case strings.Contains(upper, "PLAIN"):
		a = smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)
	case strings.Contains(upper, "LOGIN"):
		a = loginAuth{username: cfg.Username, password: cfg.Password}
	default:
		a = smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)
	}
	if err := client.Auth(a); err != nil {
		return sendErr(ReasonAuth, err)
	}
	return nil
}

func useImplicitTLS(port int) bool {
	return port == 465
}

func isLoopbackHost(host string) bool {
	host = strings.TrimSpace(host)
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(strings.Trim(host, "[]"))
	return ip != nil && ip.IsLoopback()
}

// headerSafe убирает CR и LF из значения заголовка: без этого адрес или тема
// с переводом строки дописывают в письмо свои заголовки (инъекция).
func headerSafe(v string) string {
	if !strings.ContainsAny(v, "\r\n") {
		return v
	}
	r := strings.NewReplacer("\r\n", " ", "\r", " ", "\n", " ")
	return strings.TrimSpace(r.Replace(v))
}

func envelopeAddress(from string) string {
	from = strings.TrimSpace(from)
	i := strings.LastIndex(from, "<")
	if i >= 0 {
		j := strings.LastIndex(from, ">")
		if j > i {
			return strings.TrimSpace(from[i+1 : j])
		}
	}
	return from
}

func formatAddressHeader(from string) string {
	from = strings.TrimSpace(from)
	addr := envelopeAddress(from)
	name := ""
	if i := strings.LastIndex(from, "<"); i > 0 {
		name = strings.Trim(strings.TrimSpace(from[:i]), `"`)
	}
	return (&netmail.Address{Name: name, Address: addr}).String()
}

func encodeHeader(s string) string {
	for _, r := range s {
		if r > 127 {
			return mime.QEncoding.Encode("UTF-8", s)
		}
	}
	return s
}

func messageID(from string) string {
	addr := envelopeAddress(from)
	domain := "localhost"
	if i := strings.LastIndex(addr, "@"); i >= 0 && i+1 < len(addr) {
		domain = addr[i+1:]
	}
	var rnd [8]byte
	if _, err := rand.Read(rnd[:]); err != nil {
		return ""
	}
	return fmt.Sprintf("<%d.%x@%s>", time.Now().UnixNano(), rnd, domain)
}

func buildMessage(from, to, subject, body string) []byte {
	from = headerSafe(from)
	to = headerSafe(to)
	subject = headerSafe(subject)
	var b strings.Builder
	b.WriteString("From: ")
	b.WriteString(formatAddressHeader(from))
	b.WriteString("\r\nTo: ")
	b.WriteString(to)
	b.WriteString("\r\nSubject: ")
	b.WriteString(encodeHeader(subject))
	b.WriteString("\r\nDate: ")
	b.WriteString(time.Now().UTC().Format("Mon, 02 Jan 2006 15:04:05 -0700"))
	if id := messageID(from); id != "" {
		b.WriteString("\r\nMessage-ID: ")
		b.WriteString(id)
	}
	b.WriteString("\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n")
	b.WriteString(body)
	if !strings.HasSuffix(body, "\n") {
		b.WriteString("\r\n")
	}
	return []byte(b.String())
}

func defaultDial(ctx context.Context, addr string) (net.Conn, error) {
	var d net.Dialer
	return d.DialContext(ctx, "tcp", addr)
}
