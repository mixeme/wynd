package push

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"
	"gitea.mixdep.ru/mix/wynd/internal/auth"
	"gitea.mixdep.ru/mix/wynd/internal/store"
	"gitea.mixdep.ru/mix/wynd/internal/uid"
	"gitea.mixdep.ru/mix/wynd/internal/xtime"
)

const defaultTTL = 60

// Signal is a lightweight push payload (client fetches details).
type Signal struct {
	CircleID string `json:"circle_id"`
	Type     string `json:"type"`
	Count    int    `json:"count"`
	Title    string `json:"title,omitempty"`
	Body     string `json:"body,omitempty"`
}

// SubscribeInput registers a browser push subscription.
type SubscribeInput struct {
	AccountID string
	Endpoint  string
	P256dh    string
	Auth      string
	UserAgent string
	Now       time.Time
}

// Service manages VAPID keys and web push delivery.
type Service struct {
	db     *sql.DB
	client webpush.HTTPClient
}

// New wraps a migrated SQLite store.
func New(st store.Store) (*Service, error) {
	s, ok := st.(*store.SQLite)
	if !ok {
		return nil, fmt.Errorf("push: store must be *store.SQLite")
	}
	db := s.DB()
	if db == nil {
		return nil, fmt.Errorf("push: closed store")
	}
	return &Service{db: db, client: http.DefaultClient}, nil
}

// DB exposes the underlying connection for tests.
func (s *Service) DB() *sql.DB {
	return s.db
}

// EnsureKeys generates P-256 VAPID keys when missing and stores them.
func (s *Service) EnsureKeys(ctx context.Context) error {
	pub, priv, err := s.loadKeys(ctx)
	if err != nil {
		return err
	}
	if pub != "" && priv != "" {
		return nil
	}
	priv, pub, err = webpush.GenerateVAPIDKeys()
	if err != nil {
		return fmt.Errorf("push: generate vapid keys: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `
		UPDATE instance_settings SET vapid_public_key = ?, vapid_private_key = ? WHERE id = 1
	`, pub, priv)
	if err != nil {
		return fmt.Errorf("push: save vapid keys: %w", err)
	}
	return nil
}

// PublicKey returns the instance VAPID public key.
func (s *Service) PublicKey(ctx context.Context) (string, error) {
	pub, _, err := s.loadKeys(ctx)
	if err != nil {
		return "", err
	}
	if pub == "" {
		return "", ErrNotConfigured
	}
	return pub, nil
}

// Subscribe stores or updates a push subscription for an account.
func (s *Service) Subscribe(ctx context.Context, in SubscribeInput) error {
	in.Endpoint = strings.TrimSpace(in.Endpoint)
	in.P256dh = strings.TrimSpace(in.P256dh)
	in.Auth = strings.TrimSpace(in.Auth)
	if in.AccountID == "" || in.Endpoint == "" || in.P256dh == "" || in.Auth == "" {
		return ErrInvalid
	}
	when := in.Now.UTC()
	if when.IsZero() {
		when = time.Now().UTC()
	}
	created := xtime.Format(when)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("push: begin subscribe: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var existingID, existingAccount, existingP256dh, existingAuth string
	err = tx.QueryRowContext(ctx, `
		SELECT id, account_id, p256dh, auth FROM push_subscriptions WHERE endpoint = ?
	`, in.Endpoint).Scan(&existingID, &existingAccount, &existingP256dh, &existingAuth)
	switch {
	case err == nil && existingAccount != in.AccountID &&
		(existingP256dh != in.P256dh || existingAuth != in.Auth):
		// Another account owns this endpoint. Only the browser that created
		// the subscription knows its keys, so a caller presenting different
		// keys is not that browser: refuse rather than hijack the endpoint.
		return ErrForbidden
	case err == sql.ErrNoRows:
		id, err := uid.NewID()
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `
			INSERT INTO push_subscriptions (id, account_id, endpoint, p256dh, auth, user_agent, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?)
		`, id, in.AccountID, in.Endpoint, in.P256dh, in.Auth, in.UserAgent, created)
		if err != nil {
			return fmt.Errorf("push: insert subscription: %w", err)
		}
	case err != nil:
		return fmt.Errorf("push: lookup subscription: %w", err)
	default:
		_, err = tx.ExecContext(ctx, `
			UPDATE push_subscriptions
			SET account_id = ?, p256dh = ?, auth = ?, user_agent = ?
			WHERE endpoint = ?
		`, in.AccountID, in.P256dh, in.Auth, in.UserAgent, in.Endpoint)
		if err != nil {
			return fmt.Errorf("push: update subscription: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("push: commit subscribe: %w", err)
	}
	return nil
}

// Unsubscribe removes a push subscription for an account.
func (s *Service) Unsubscribe(ctx context.Context, accountID, endpoint string) error {
	endpoint = strings.TrimSpace(endpoint)
	if accountID == "" || endpoint == "" {
		return ErrInvalid
	}
	res, err := s.db.ExecContext(ctx, `
		DELETE FROM push_subscriptions WHERE account_id = ? AND endpoint = ?
	`, accountID, endpoint)
	if err != nil {
		return fmt.Errorf("push: unsubscribe: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("push: unsubscribe rows: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// SendSignal delivers a signal payload to all subscriptions of an account.
func (s *Service) SendSignal(ctx context.Context, accountID string, signal Signal) error {
	if accountID == "" || strings.TrimSpace(signal.Type) == "" {
		return ErrInvalid
	}
	pub, priv, err := s.loadKeys(ctx)
	if err != nil {
		return err
	}
	if pub == "" || priv == "" {
		return ErrNotConfigured
	}

	payload, err := json.Marshal(signal)
	if err != nil {
		return fmt.Errorf("push: marshal signal: %w", err)
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, endpoint, p256dh, auth FROM push_subscriptions WHERE account_id = ?
	`, accountID)
	if err != nil {
		return fmt.Errorf("push: list subscriptions: %w", err)
	}
	defer rows.Close()

	var firstErr error
	for rows.Next() {
		var id, endpoint, p256dh, authKey string
		if err := rows.Scan(&id, &endpoint, &p256dh, &authKey); err != nil {
			return fmt.Errorf("push: scan subscription: %w", err)
		}
		if err := s.deliver(ctx, pub, priv, endpoint, p256dh, authKey, payload); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			if isStaleSubscription(err) {
				_, _ = s.db.ExecContext(ctx, `DELETE FROM push_subscriptions WHERE id = ?`, id)
			}
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("push: subscriptions: %w", err)
	}
	return firstErr
}

// SendTest sends a test notification to all subscriptions of an account.
func (s *Service) SendTest(ctx context.Context, accountID string) error {
	return s.SendSignal(ctx, accountID, Signal{
		Type:  "test",
		Count: 0,
		Title: "Проверка уведомлений Wynd",
		Body:  "Если вы видите это сообщение, push-уведомления работают.",
	})
}

func (s *Service) deliver(ctx context.Context, pub, priv, endpoint, p256dh, authKey string, payload []byte) error {
	sub := &webpush.Subscription{
		Endpoint: endpoint,
		Keys: webpush.Keys{
			Auth:   authKey,
			P256dh: p256dh,
		},
	}
	resp, err := webpush.SendNotificationWithContext(ctx, payload, sub, &webpush.Options{
		HTTPClient:      s.client,
		Subscriber:      auth.AdminSentinelEmail,
		TTL:             defaultTTL,
		VAPIDPublicKey:  pub,
		VAPIDPrivateKey: priv,
	})
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	return &deliveryError{StatusCode: resp.StatusCode}
}

func (s *Service) loadKeys(ctx context.Context) (public, private string, err error) {
	err = s.db.QueryRowContext(ctx, `
		SELECT vapid_public_key, vapid_private_key FROM instance_settings WHERE id = 1
	`).Scan(&public, &private)
	if err != nil {
		return "", "", fmt.Errorf("push: load vapid keys: %w", err)
	}
	return public, private, nil
}

func isStaleSubscription(err error) bool {
	var de *deliveryError
	return errors.As(err, &de) && (de.StatusCode == http.StatusGone || de.StatusCode == http.StatusNotFound)
}

type deliveryError struct {
	StatusCode int
}

func (e *deliveryError) Error() string {
	return fmt.Sprintf("push: delivery failed: status %d", e.StatusCode)
}
