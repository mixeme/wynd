package chronicle_test

import (
	"testing"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
)

// Итог переноса хранит «уже здесь» (C51): экран 7.25 говорит «Все уже на
// этом сервере», как после оживления, и после перезапуска тоже.
func TestIncomingResultKeepsAlreadyHere(t *testing.T) {
	ch := newTestChronicle(t)
	ctx := t.Context()
	now := time.Date(2026, 10, 23, 10, 0, 0, 0, time.UTC)
	if _, err := ch.DB().ExecContext(ctx, `INSERT INTO accounts (id, email, created_at) VALUES ('kot', 'kot@test.local', '2026-10-23T10:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	m, err := ch.PutIncomingDraft(ctx, "kot", "https://old.example/move/AAAA", chronicle.IncomingCard{
		SourceURL: "https://old.example", CircleName: "Семья", ExpiresAt: now.Add(time.Hour),
	}, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := ch.SetIncomingRevived(ctx, m.ID, "c1", 3, 2, 100, true, 0, 0, 1); err != nil {
		t.Fatal(err)
	}
	got, err := ch.GetIncoming(ctx, m.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.AlreadyHere != 1 || got.Invited != 0 || got.NotJoined != 0 {
		t.Fatalf("итог: уже здесь %d, позвали %d, не вошли %d", got.AlreadyHere, got.Invited, got.NotJoined)
	}
}
