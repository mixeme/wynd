package auth

import (
	"context"
	"testing"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/store"
)

// Порог записи активности (белый ящик): второй touch в пределах
// sessionTouchEvery не пишет ни last_seen_at, ни device.
func TestTouchSessionThrottled(t *testing.T) {
	st, err := store.OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	db := st.(*store.SQLite).DB()
	s := &Service{db: db, touchSeen: map[string]time.Time{}}
	ctx := context.Background()

	if _, err := db.Exec(`INSERT INTO accounts (id, email, created_at) VALUES ('acc1', 'acc1@example.com', '2026-01-01T00:00:00Z')`); err != nil {
		t.Fatalf("insert account: %v", err)
	}
	if _, err := db.Exec(`
		INSERT INTO sessions (token_hash, account_id, kind, expires_at, created_at, device, last_seen_at)
		VALUES (?, 'acc1', 'participant', '2030-01-01T00:00:00Z', '2026-01-01T00:00:00Z', 'Edge на Windows', '2026-01-01T00:00:00Z')
	`, hashSessionToken("tok")); err != nil {
		t.Fatalf("insert: %v", err)
	}
	row := `SELECT last_seen_at, device FROM sessions WHERE token_hash = ?`

	if err := s.TouchSession(ctx, "tok", "Firefox на Windows"); err != nil {
		t.Fatalf("touch: %v", err)
	}
	var ls, dev string
	if err := db.QueryRow(row, hashSessionToken("tok")).Scan(&ls, &dev); err != nil {
		t.Fatal(err)
	}

	// Второй touch сразу — порог не вышел, ничего не поменялось.
	if err := s.TouchSession(ctx, "tok", "Chrome на macOS"); err != nil {
		t.Fatalf("touch 2: %v", err)
	}
	var ls2, dev2 string
	if err := db.QueryRow(row, hashSessionToken("tok")).Scan(&ls2, &dev2); err != nil {
		t.Fatal(err)
	}
	if ls2 != ls || dev2 != dev {
		t.Fatalf("порог не сработал: %q→%q, %q→%q", ls, ls2, dev, dev2)
	}

	// Сброс порога имитируем чисткой карты — дальше запись проходит.
	// (last_seen_at может не измениться видимо: formatTime с точностью до
	// секунды, оба touch в одной секунде — доказательство в device.)
	s.touchSeen = map[string]time.Time{}
	if err := s.TouchSession(ctx, "tok", "Chrome на macOS"); err != nil {
		t.Fatalf("touch 3: %v", err)
	}
	if err := db.QueryRow(row, hashSessionToken("tok")).Scan(&ls2, &dev2); err != nil {
		t.Fatal(err)
	}
	if dev2 != "Chrome на macOS" || ls2 < ls {
		t.Fatalf("после сброса порога запись не прошла: %q, %q", ls2, dev2)
	}
}
