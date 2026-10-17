package api_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/api"
)

func startArchive(t *testing.T, srv *api.Server, tok, circleID string) {
	t.Helper()
	now := time.Now().UTC()
	rec := doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+circleID+"/archive", tok, map[string]any{
		"cutoff_date":         now.Add(24 * time.Hour).Format("2006-01-02"),
		"deadline":            now.Add(48 * time.Hour).Format(time.RFC3339),
		"reminder_before_sec": 3600,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("start archive: %d %s", rec.Code, rec.Body.String())
	}
}

// Инвариант (план 42, ARC-11; A7 — 2026-10-17): без цикла свой архив отдаётся целиком,
// 429 с Retry-After, пока у учётки идёт другая сборка, 403 после срока.
func TestArchiveDownloadStatuses(t *testing.T) {
	srv, caps, ch, _ := setupAPI(t)
	tok, accountID := registerSession(t, srv, caps, "statuses@example.com")
	rec := doJSON(t, srv, http.MethodPost, "/api/v1/circles", tok, map[string]any{
		"name": "Семья", "owner_name": "Аня",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("circle: %d %s", rec.Code, rec.Body.String())
	}
	circleID := jsonStr(t, rec, "id")
	download := "/api/v1/circles/" + circleID + "/archive/download"

	if r := doGET(t, srv, download, tok); r.Code != http.StatusOK {
		t.Fatalf("без цикла: %d %s", r.Code, r.Body.String())
	}
	// Вне цикла скачивание ничего не фиксирует: отсечка потом свободно ставится.
	var locked int
	if err := ch.DB().QueryRowContext(t.Context(), `SELECT COUNT(*) FROM circles WHERE id = ? AND cutoff_locked_at IS NOT NULL`, circleID).Scan(&locked); err != nil || locked != 0 {
		t.Fatalf("отсечка зафиксирована без цикла: %v %d", err, locked)
	}

	startArchive(t, srv, tok, circleID)
	release := srv.HoldArchiveBuildForTest(accountID)
	r := doGET(t, srv, download, tok)
	release()
	if r.Code != http.StatusTooManyRequests || r.Header().Get("Retry-After") == "" {
		t.Fatalf("параллельная сборка: %d %q", r.Code, r.Header().Get("Retry-After"))
	}
	if r := doGET(t, srv, download, tok); r.Code != http.StatusOK {
		t.Fatalf("после сборки: %d %s", r.Code, r.Body.String())
	}

	if _, err := ch.DB().ExecContext(t.Context(), `
		UPDATE circles SET archive_deadline = '2026-01-01T00:00:00.000000000Z' WHERE id = ?
	`, circleID); err != nil {
		t.Fatal(err)
	}
	if r := doGET(t, srv, download, tok); r.Code != http.StatusForbidden {
		t.Fatalf("после срока: %d %s", r.Code, r.Body.String())
	}
}

// Инвариант (ARC-11): исключённый из круга архив не скачивает.
func TestArchiveDownloadForbiddenForExcluded(t *testing.T) {
	srv, caps, _, _ := setupAPI(t)
	ownerTok, memberTok, circleID, memberID := circleWithMember(t, srv, caps)
	startArchive(t, srv, ownerTok, circleID)
	rec := doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+circleID+"/exclude", ownerTok, map[string]any{
		"account_id": memberID,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("исключение: %d %s", rec.Code, rec.Body.String())
	}
	if r := doGET(t, srv, "/api/v1/circles/"+circleID+"/archive/download", memberTok); r.Code != http.StatusForbidden {
		t.Fatalf("исключённый: %d %s", r.Code, r.Body.String())
	}
}

func cutoffLocked(t *testing.T, srv *api.Server, tok, circleID string) bool {
	t.Helper()
	rec := doGET(t, srv, "/api/v1/circles/"+circleID, tok)
	if rec.Code != http.StatusOK {
		t.Fatalf("circle: %d %s", rec.Code, rec.Body.String())
	}
	var out struct {
		ArchiveCycle struct {
			CutoffLocked bool `json:"cutoff_locked"`
		} `json:"archive_cycle"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out.ArchiveCycle.CutoffLocked
}

// brokenWriter — клиент, у которого связь рвётся после limit байт тела.
type brokenWriter struct {
	*httptest.ResponseRecorder
	limit int
}

func (b *brokenWriter) Write(p []byte) (int, error) {
	if b.Body.Len()+len(p) > b.limit {
		return 0, errors.New("connection reset")
	}
	return b.ResponseRecorder.Write(p)
}

func brokenGET(t *testing.T, srv *api.Server, path, tok string, limit int) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	defer func() {
		if r := recover(); r != nil && r != http.ErrAbortHandler {
			panic(r)
		}
	}()
	srv.ServeHTTP(&brokenWriter{ResponseRecorder: httptest.NewRecorder(), limit: limit}, req)
}

// Инвариант (план 42, ARC-8): отсечка замирает, когда архив отдан до
// последнего байта, а не когда собран: оборванная отдача её не фиксирует,
// отдача целиком — фиксирует.
func TestArchiveCutoffLocksOnlyAfterFullDelivery(t *testing.T) {
	srv, caps, _, _ := setupAPI(t)
	tok, _ := registerSession(t, srv, caps, "lock@example.com")
	rec := doJSON(t, srv, http.MethodPost, "/api/v1/circles", tok, map[string]any{
		"name": "Семья", "owner_name": "Аня",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("circle: %d %s", rec.Code, rec.Body.String())
	}
	circleID := jsonStr(t, rec, "id")
	startArchive(t, srv, tok, circleID)
	download := "/api/v1/circles/" + circleID + "/archive/download"

	brokenGET(t, srv, download, tok, 10)
	if cutoffLocked(t, srv, tok, circleID) {
		t.Fatal("отсечка замерла после оборванной отдачи")
	}
	downloadArchive(t, srv, tok, circleID)
	if !cutoffLocked(t, srv, tok, circleID) {
		t.Fatal("отсечка не замерла после отдачи до конца")
	}
}
