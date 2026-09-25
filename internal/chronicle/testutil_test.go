package chronicle_test

import (
	"context"
	"testing"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
	"gitea.mixdep.ru/mix/wynd/internal/store"
)

type testEnv struct {
	ch  *chronicle.Chronicle
	ctx context.Context
	t   *testing.T
	t0  time.Time
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	st, err := store.OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ch, err := chronicle.New(st)
	if err != nil {
		t.Fatalf("chronicle.New: %v", err)
	}
	base := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	return &testEnv{ch: ch, ctx: context.Background(), t: t, t0: base}
}

func (e *testEnv) at(days int) time.Time {
	return e.t0.Add(time.Duration(days) * 24 * time.Hour)
}

func (e *testEnv) after(d time.Duration) time.Time {
	return e.t0.Add(d)
}

func (e *testEnv) createCircle(ownerID, ownerName string, window chronicle.EditWindow) chronicle.Circle {
	circle, _, _, err := e.ch.CreateCircle(e.ctx, chronicle.CreateCircleInput{
		Name:           "Семья",
		OwnerAccountID: ownerID,
		OwnerName:      ownerName,
		EditWindow:     window,
		Now:            e.t0,
	})
	if err != nil {
		e.t.Fatalf("CreateCircle: %v", err)
	}
	return circle
}

func (e *testEnv) join(circleID, accountID, name string, when time.Time) {
	if _, _, err := e.ch.Join(e.ctx, chronicle.JoinInput{
		CircleID: circleID, AccountID: accountID, Name: name, Now: when,
	}); err != nil {
		e.t.Fatalf("Join %s: %v", accountID, err)
	}
}

func (e *testEnv) post(circleID, accountID, body, entryDate string, when time.Time) chronicle.Post {
	p, err := e.ch.CreatePost(e.ctx, chronicle.PostInput{
		CircleID: circleID, AccountID: accountID, Body: body, EntryDate: entryDate, Now: when,
	})
	if err != nil {
		e.t.Fatalf("CreatePost: %v", err)
	}
	return p
}

func (e *testEnv) seedBlob(id, accountID string) {
	e.seedAccount(accountID)
	_, err := e.ch.DB().ExecContext(e.ctx, `
		INSERT INTO blobs (id, account_id, sha256, size_bytes, mime_type, storage_path, status, created_at)
		VALUES (?, ?, 'deadbeef', 1, 'image/jpeg', 'de/ad', 'complete', ?)
	`, id, accountID, e.t0.UTC().Format(time.RFC3339))
	if err != nil {
		e.t.Fatalf("seedBlob %s: %v", id, err)
	}
}

func (e *testEnv) attachPhoto(postID, blobID string) {
	if err := e.ch.AttachMedia(e.ctx, postID, []chronicle.MediaInput{{
		BlobID: blobID, Kind: chronicle.MediaPhoto,
	}}); err != nil {
		e.t.Fatalf("AttachMedia: %v", err)
	}
}

func (e *testEnv) seedAccount(accountID string) {
	_, err := e.ch.DB().ExecContext(e.ctx, `
		INSERT OR IGNORE INTO accounts (id, email, created_at) VALUES (?, ?, ?)
	`, accountID, accountID+"@test.local", e.t0.UTC().Format(time.RFC3339))
	if err != nil {
		e.t.Fatalf("seedAccount %s: %v", accountID, err)
	}
}

func eventTextRemains(e *testEnv, circleID, needle string) (bool, error) {
	rows, err := e.ch.DB().QueryContext(e.ctx, `
		SELECT payload, summary FROM events WHERE circle_id = ?
	`, circleID)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var payload, summary string
		if err := rows.Scan(&payload, &summary); err != nil {
			return false, err
		}
		if containsFold(payload, needle) || containsFold(summary, needle) {
			return true, nil
		}
	}
	return false, rows.Err()
}

func containsFold(s, needle string) bool {
	return len(s) >= len(needle) && (s == needle || len(needle) == 0 ||
		stringIndex(s, needle) >= 0)
}

func stringIndex(s, needle string) int {
	n := len(needle)
	for i := 0; i+n <= len(s); i++ {
		if s[i:i+n] == needle {
			return i
		}
	}
	return -1
}
