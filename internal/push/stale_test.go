package push

import (
	"context"
	"testing"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/store"
)

// Инвариант (C128): мёртвая подписка снимается, даже если бюджет рассылки уже
// исчерпан и контекст отменён. Иначе адрес остаётся в базе, и каждый следующий
// сигнал снова получает 410, а причина удаления нигде не видна.
func TestDropStaleSurvivesCanceledBudget(t *testing.T) {
	st, err := store.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	svc, err := New(st)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := svc.db.ExecContext(ctx, `
		INSERT INTO accounts (id, email, created_at) VALUES ('acc', 'a@test.local', ?)
	`, now); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.db.ExecContext(ctx, `
		INSERT INTO push_subscriptions (id, account_id, endpoint, p256dh, auth, user_agent, created_at)
		VALUES ('s', 'acc', 'https://push.example/ep', 'p256dh', 'auth', '', ?)
	`, now); err != nil {
		t.Fatal(err)
	}

	canceled, cancel := context.WithCancel(ctx)
	cancel()
	svc.dropStale(canceled, "s", "https://push.example/ep")

	var left int
	if err := svc.db.QueryRowContext(ctx,
		`SELECT count(*) FROM push_subscriptions WHERE id = 's'`).Scan(&left); err != nil {
		t.Fatal(err)
	}
	if left != 0 {
		t.Fatalf("мёртвая подписка осталась: %d", left)
	}
}
