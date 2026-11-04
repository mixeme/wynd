package api

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/auth"
	"gitea.mixdep.ru/mix/wynd/internal/blob"
	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
	"gitea.mixdep.ru/mix/wynd/internal/store"
)

type budgetTransport func(*http.Request) (*http.Response, error)

func (f budgetTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type countedMoveBody struct{ reads atomic.Int32 }

func (b *countedMoveBody) Read(p []byte) (int, error) { b.reads.Add(1); return 0, io.EOF }
func (*countedMoveBody) Close() error                 { return nil }

func TestMoveBudgetUsesActualLengthAndPinsAcrossRestart(t *testing.T) {
	dir := t.TempDir()
	s := &Server{DataDir: dir, MoveMaxArchiveBytes: 8, MoveDiskFree: func() (int64, error) { return 100, nil }}
	if err := os.MkdirAll(MoveDir(dir), 0700); err != nil {
		t.Fatal(err)
	}
	body := &countedMoveBody{}
	s.MoveClient = &http.Client{Transport: budgetTransport(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 206, Header: http.Header{"Content-Range": []string{"bytes 0-3/16"}}, Body: body}, nil
	})}
	_, _, err := s.pullMoveChunk(t.Context(), &chronicle.IncomingMove{ID: "small", Bytes: 1, Link: "https://source.example/move/AAAA-BBBB"}, s.incomingPath("small"), 0)
	var failed *moveFailure
	if !errors.As(err, &failed) || failed.code != "no_room" || body.reads.Load() != 0 {
		t.Fatalf("length budget: %v reads=%d", err, body.reads.Load())
	}
	if err := s.pinMoveTotal("stable", 4); err != nil {
		t.Fatal(err)
	}
	restarted := &Server{DataDir: dir, MoveMaxArchiveBytes: 8}
	if err := restarted.pinMoveTotal("stable", 5); !errors.As(err, &failed) || failed.code != "archive_changed" {
		t.Fatalf("length changed across restart: %v", err)
	}
	if err := restarted.pinMoveTotal("stable", 4); err != nil {
		t.Fatal(err)
	}
}

func TestMoveReservationsCannotOverbookDisk(t *testing.T) {
	s := &Server{DataDir: t.TempDir(), MoveDiskFree: func() (int64, error) { return 10, nil }}
	if err := s.reserveMoveDisk(t.Context(), "a", 6); err != nil {
		t.Fatal(err)
	}
	if err := s.reserveMoveDisk(t.Context(), "b", 5); err == nil {
		t.Fatal("overbooked disk")
	}
	if err := s.reserveMoveDisk(t.Context(), "a", 2); err != nil {
		t.Fatal(err)
	}
	if err := s.reserveMoveDisk(t.Context(), "b", 5); err != nil {
		t.Fatal(err)
	}
}

func TestMoveImportWaitObservesCancellation(t *testing.T) {
	s := &Server{}
	s.moveImportOnce.Do(func() { s.moveImportGate = make(chan struct{}, 1) })
	s.moveImportGate <- struct{}{}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := s.reviveMoveFile(ctx, &chronicle.IncomingMove{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("import wait ignored cancellation: %v", err)
	}
}

func budgetTestServer(t *testing.T) *Server {
	t.Helper()
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "wynd.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	ch, err := chronicle.New(st)
	if err != nil {
		t.Fatal(err)
	}
	blobs, err := blob.New(st, filepath.Join(dir, "blobs"))
	if err != nil {
		t.Fatal(err)
	}
	a, err := auth.New(st, ch, auth.NewCaptureCodes(), true)
	if err != nil {
		t.Fatal(err)
	}
	return &Server{Chronicle: ch, Blobs: blobs, Auth: a, DataDir: dir, MoveRetry: time.Millisecond, MoveTimeout: 100 * time.Millisecond}
}

func budgetTestDraft(t *testing.T, s *Server, id string) *chronicle.IncomingMove {
	t.Helper()
	_, err := s.Chronicle.DB().Exec(`INSERT INTO accounts(id,email,created_at) VALUES(?,?,?)`, id, id+"@example.com", time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		t.Fatal(err)
	}
	m, err := s.Chronicle.PutIncomingDraft(t.Context(), id, "https://source.example/move/AAAA-BBBB", chronicle.IncomingCard{CircleName: "source", ExpiresAt: time.Now().Add(time.Hour), Bytes: 1}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Chronicle.StartIncoming(t.Context(), m.ID, false, "", time.Now()); err != nil {
		t.Fatal(err)
	}
	return m
}

func waitBudgetWorkers(t *testing.T, s *Server) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		s.moveBudgetMu.Lock()
		n := s.moveActive
		reserved := len(s.moveReservations)
		s.moveBudgetMu.Unlock()
		if n == 0 {
			if reserved != 0 {
				t.Fatal("reservation leaked")
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("worker did not release resources")
		}
		time.Sleep(time.Millisecond)
	}
}

func TestMoveNoRoomStopsBeforeBodyAndReleasesResources(t *testing.T) {
	s := budgetTestServer(t)
	s.MoveDiskFree = func() (int64, error) { return 8, nil }
	s.MoveTimeout = time.Second
	body := &countedMoveBody{}
	s.MoveClient = &http.Client{Transport: budgetTransport(func(r *http.Request) (*http.Response, error) {
		if strings.HasSuffix(r.URL.Path, "/claim") {
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("{}"))}, nil
		}
		return &http.Response{StatusCode: 206, Header: http.Header{"Content-Range": []string{"bytes 0-3/16"}}, Body: body}, nil
	})}
	m := budgetTestDraft(t, s, "owner")
	s.runIncomingMove(m.ID)
	waitBudgetWorkers(t, s)
	m, err := s.Chronicle.GetIncoming(t.Context(), m.ID)
	if err != nil || m.Status != chronicle.IncomingFailed || m.Error != "no_room" {
		t.Fatalf("worker: %+v %v", m, err)
	}
	if body.reads.Load() != 0 {
		t.Fatal("download started without disk budget")
	}
	for _, path := range []string{s.incomingPath(m.ID), s.incomingPath(m.ID) + ".range"} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("temporary file retained: %s %v", path, err)
		}
	}
}

func TestMoveWorkersHaveCountAndTimeLimits(t *testing.T) {
	s := budgetTestServer(t)
	s.MoveClient = &http.Client{Transport: budgetTransport(func(r *http.Request) (*http.Response, error) { <-r.Context().Done(); return nil, r.Context().Err() })}
	var ids []string
	for i := 0; i < 3; i++ {
		m := budgetTestDraft(t, s, fmt.Sprint("owner", i))
		ids = append(ids, m.ID)
	}
	for _, id := range ids {
		s.runIncomingMove(id)
	}
	third, err := s.Chronicle.GetIncoming(t.Context(), ids[2])
	if err != nil || third.Error != "move_busy" {
		t.Fatalf("third job admitted: %+v %v", third, err)
	}
	waitBudgetWorkers(t, s)
	for _, id := range ids[:2] {
		m, err := s.Chronicle.GetIncoming(context.Background(), id)
		if err != nil || m.Status != chronicle.IncomingFailed {
			t.Fatalf("deadline ignored: %+v %v", m, err)
		}
	}
}
