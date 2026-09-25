package api_test

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/api"
	"gitea.mixdep.ru/mix/wynd/internal/archive"
	"gitea.mixdep.ru/mix/wynd/internal/auth"
	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
	"gitea.mixdep.ru/mix/wynd/internal/jobs"
)

func TestAcceptanceCircleInviteSyncLeaveDelete(t *testing.T) {
	srv, caps, ch, _ := setupAPI(t)
	ownerTok, _ := registerSession(t, srv, caps, "anya@example.com")

	rec := doJSON(t, srv, http.MethodPost, "/api/v1/circles", ownerTok, map[string]any{
		"name": "Семья", "owner_name": "Аня",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create circle: %d %s", rec.Code, rec.Body.String())
	}
	circleID := jsonStr(t, rec, "id")

	allowCircleMultiInvites(t, srv, circleID, ownerTok)
	rec = doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+circleID+"/invites", ownerTok, map[string]any{
		"kind": "multi", "max_uses": 5, "ttl_sec": 7 * 86400,
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("invite: %d %s", rec.Code, rec.Body.String())
	}
	inviteToken := jsonStr(t, rec, "token")

	bobTok := acceptInvite(t, srv, caps, inviteToken, "bob@example.com", "Боб")

	secret := "секретный текст ветки"
	rec = doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+circleID+"/posts", ownerTok, map[string]any{
		"body": secret, "entry_date": "2026-08-30",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("post: %d %s", rec.Code, rec.Body.String())
	}
	postID := jsonStr(t, rec, "id")

	bobEvents := syncEvents(t, srv, bobTok)
	if !eventsContain(bobEvents, secret) {
		t.Fatal("bob sync must include the post")
	}

	rec = doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+circleID+"/leave", bobTok, map[string]any{
		"retain_access": true,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("leave: %d %s", rec.Code, rec.Body.String())
	}

	bobEvents = syncEvents(t, srv, bobTok)
	if !eventsContain(bobEvents, secret) {
		t.Fatal("left with access must still see the post")
	}
	rec = doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+circleID+"/posts", bobTok, map[string]any{
		"body": "после выхода", "entry_date": "2026-08-30",
	})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("left with access must not write: %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+circleID+"/invites", ownerTok, map[string]any{
		"kind": "single", "max_uses": 1,
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("second invite: %d %s", rec.Code, rec.Body.String())
	}
	veraTok := acceptInvite(t, srv, caps, jsonStr(t, rec, "token"), "vera@example.com", "Вера")

	veraEvents := syncEvents(t, srv, veraTok)
	if eventsContain(veraEvents, secret) {
		t.Fatal("third member must not see the past")
	}

	rec = doJSON(t, srv, http.MethodDelete, "/api/v1/circles/"+circleID+"/posts/"+postID, ownerTok, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("delete: %d %s", rec.Code, rec.Body.String())
	}
	if remains, err := eventTextRemainsDB(t, ch, circleID, secret); err != nil || remains {
		t.Fatalf("text remains in events: remains=%v err=%v", remains, err)
	}
}

func TestAcceptanceDaysBackdateTitleCoverCollapse(t *testing.T) {
	srv, caps, ch, _ := setupAPI(t)
	tok, _ := registerSession(t, srv, caps, "days@example.com")
	rec := doJSON(t, srv, http.MethodPost, "/api/v1/circles", tok, map[string]any{
		"name": "Семья", "owner_name": "Аня",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("circle: %d %s", rec.Code, rec.Body.String())
	}
	circleID := jsonStr(t, rec, "id")
	blobID := uploadBytes(t, srv, tok, []byte("cover-bytes"), "image/jpeg")

	rec = doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+circleID+"/posts", tok, map[string]any{
		"body": "задним числом", "entry_date": "2026-08-01",
		"media": []map[string]any{{"blob_id": blobID, "kind": "photo", "is_cover": true}},
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("backdated post: %d %s", rec.Code, rec.Body.String())
	}
	postID := jsonStr(t, rec, "id")

	rec = doGET(t, srv, "/api/v1/circles/"+circleID+"/days", tok)
	if rec.Code != http.StatusOK {
		t.Fatalf("days: %d %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "2026-08-01") {
		t.Fatalf("day missing: %s", rec.Body.String())
	}

	rec = doJSON(t, srv, http.MethodPut, "/api/v1/circles/"+circleID+"/days/2026-08-01/title", tok, map[string]string{
		"title": "Первый день",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("title: %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, srv, http.MethodPut, "/api/v1/circles/"+circleID+"/days/2026-08-01/cover", tok, map[string]string{
		"post_id": postID, "blob_id": blobID,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("cover: %d %s", rec.Code, rec.Body.String())
	}

	rec = doGET(t, srv, "/api/v1/circles/"+circleID+"/days", tok)
	body := rec.Body.String()
	if !strings.Contains(body, "Первый день") || !strings.Contains(body, blobID) {
		t.Fatalf("titled day: %s", body)
	}

	rec = doJSON(t, srv, http.MethodDelete, "/api/v1/circles/"+circleID+"/posts/"+postID, tok, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("delete post: %d %s", rec.Code, rec.Body.String())
	}

	rec = doGET(t, srv, "/api/v1/circles/"+circleID+"/days", tok)
	if rec.Code != http.StatusOK {
		t.Fatalf("days after collapse: %d %s", rec.Code, rec.Body.String())
	}
	var days map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &days); err != nil {
		t.Fatal(err)
	}
	list, _ := days["days"].([]any)
	if len(list) != 0 {
		t.Fatalf("day should collapse: %#v", list)
	}
	if remains, err := eventTextRemainsDB(t, ch, circleID, "Первый день"); err != nil || remains {
		t.Fatalf("day title remains: remains=%v err=%v", remains, err)
	}
	if remains, err := eventTextRemainsDB(t, ch, circleID, "задним числом"); err != nil || remains {
		t.Fatalf("post text remains: remains=%v err=%v", remains, err)
	}
}

func TestAcceptanceQuotaArchiveLockPurge(t *testing.T) {
	srv, caps, ch, blobs := setupAPI(t)
	ownerTok, _ := registerSession(t, srv, caps, "owner@example.com")
	rec := doJSON(t, srv, http.MethodPost, "/api/v1/circles", ownerTok, map[string]any{
		"name": "Семья", "owner_name": "Аня",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("circle: %d %s", rec.Code, rec.Body.String())
	}
	circleID := jsonStr(t, rec, "id")

	early := []byte("early-photo")
	earlyID := uploadBytes(t, srv, ownerTok, early, "image/jpeg")
	rec = doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+circleID+"/posts", ownerTok, map[string]any{
		"body": "ранняя", "entry_date": "2026-08-01",
		"media": []map[string]any{{"blob_id": earlyID, "kind": "photo", "is_cover": true}},
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("early post: %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+circleID+"/invites", ownerTok, map[string]any{
		"kind": "single",
	})
	guestTok := acceptInvite(t, srv, caps, jsonStr(t, rec, "token"), "guest@example.com", "Боря")

	late := []byte("late-photo")
	lateID := uploadBytes(t, srv, guestTok, late, "image/jpeg")
	rec = doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+circleID+"/posts", guestTok, map[string]any{
		"body": "после входа", "entry_date": "2026-08-20",
		"media": []map[string]any{{"blob_id": lateID, "kind": "photo", "is_cover": true}},
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("guest post: %d %s", rec.Code, rec.Body.String())
	}

	if _, err := blobs.DB().ExecContext(t.Context(), `UPDATE circles SET quota_custom = 1, quota_bytes = 1 WHERE id = ?`, circleID); err != nil {
		t.Fatal(err)
	}
	blocked := uploadBytes(t, srv, ownerTok, []byte("too-big-for-circle"), "image/jpeg")
	rec = doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+circleID+"/posts", ownerTok, map[string]any{
		"body": "", "entry_date": "2026-08-21",
		"media": []map[string]any{{"blob_id": blocked, "kind": "photo", "is_cover": true}},
	})
	if rec.Code != http.StatusInsufficientStorage {
		t.Fatalf("quota hit: %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+circleID+"/posts", ownerTok, map[string]any{
		"body": "только текст", "entry_date": "2026-08-21",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("text at quota: %d %s", rec.Code, rec.Body.String())
	}

	now := time.Now().UTC()
	deadline := now.Add(time.Hour)
	cutoff := now.Add(24 * time.Hour).Format("2006-01-02")
	rec = doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+circleID+"/archive", ownerTok, map[string]any{
		"cutoff_date":         cutoff,
		"deadline":            deadline.Format(time.RFC3339),
		"reminder_before_sec": 3600,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("start archive: %d %s", rec.Code, rec.Body.String())
	}

	ownerZip := downloadArchive(t, srv, ownerTok, circleID)
	guestZip := downloadArchive(t, srv, guestTok, circleID)
	if bytes.Equal(ownerZip, guestZip) {
		t.Fatal("archives of two members with different spans must differ")
	}
	assertOfflineZIP(t, ownerZip)
	assertOfflineZIP(t, guestZip)

	rec = doJSON(t, srv, http.MethodPut, "/api/v1/circles/"+circleID+"/archive/cutoff", ownerTok, map[string]any{
		"cutoff_date": now.Add(48 * time.Hour).Format("2006-01-02"),
	})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("locked cutoff: %d %s", rec.Code, rec.Body.String())
	}

	counts, err := jobs.RunArchiveJobs(t.Context(), ch.DB(), ch, blobs, srv.Mail, "http://127.0.0.1:7676", deadline.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if counts.PurgedCircles != 1 {
		t.Fatalf("purged: %+v", counts)
	}
	if remains, err := eventTextRemainsDB(t, ch, circleID, "ранняя"); err != nil || remains {
		t.Fatalf("said content after purge: remains=%v err=%v", remains, err)
	}
	var serviceCount int
	if err := ch.DB().QueryRowContext(t.Context(), `
		SELECT COUNT(*) FROM events WHERE circle_id = ? AND is_service = 1
	`, circleID).Scan(&serviceCount); err != nil || serviceCount < 1 {
		t.Fatalf("structural events: count=%d err=%v", serviceCount, err)
	}
}

func doJSON(t *testing.T, srv *api.Server, method, path, token string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var rdr io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		rdr = bytes.NewReader(raw)
	}
	req := httptest.NewRequest(method, path, rdr)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	return rec
}

func doGET(t *testing.T, srv *api.Server, path, token string) *httptest.ResponseRecorder {
	t.Helper()
	return doJSON(t, srv, http.MethodGet, path, token, nil)
}

func jsonStr(t *testing.T, rec *httptest.ResponseRecorder, key string) string {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("json: %v body=%s", err, rec.Body.String())
	}
	s, ok := out[key].(string)
	if !ok || s == "" {
		t.Fatalf("missing %s in %s", key, rec.Body.String())
	}
	return s
}

func acceptInvite(t *testing.T, srv *api.Server, caps *auth.CaptureCodes, token, email, name string) string {
	t.Helper()
	rec := doJSON(t, srv, http.MethodPost, "/api/v1/invites/"+token+"/accept", "", map[string]string{
		"email": email, "name": name,
	})
	if rec.Code != http.StatusAccepted {
		t.Fatalf("accept: %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, srv, http.MethodPost, "/api/v1/auth/verify", "", map[string]string{
		"email": email, "code": caps.Last(email),
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("verify: %d %s", rec.Code, rec.Body.String())
	}
	return jsonStr(t, rec, "token")
}

func syncEvents(t *testing.T, srv *api.Server, token string) []any {
	t.Helper()
	rec := doGET(t, srv, "/api/v1/sync?cursor=0", token)
	if rec.Code != http.StatusOK {
		t.Fatalf("sync: %d %s", rec.Code, rec.Body.String())
	}
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	events, _ := out["events"].([]any)
	return events
}

func eventsContain(events []any, needle string) bool {
	raw, _ := json.Marshal(events)
	return strings.Contains(string(raw), needle)
}

func uploadBytes(t *testing.T, srv *api.Server, token string, payload []byte, mime string) string {
	t.Helper()
	rec := doJSON(t, srv, http.MethodPost, "/api/v1/uploads", token, map[string]any{
		"expected_size": len(payload), "mime_type": mime,
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("upload session: %d %s", rec.Code, rec.Body.String())
	}
	sessionID := jsonStr(t, rec, "id")
	req := httptest.NewRequest(http.MethodPut, "/api/v1/uploads/"+sessionID, bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Upload-Offset", "0")
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("chunk: %d %s", rec.Code, rec.Body.String())
	}
	sum := sha256.Sum256(payload)
	rec = doJSON(t, srv, http.MethodPost, "/api/v1/uploads/"+sessionID+"/complete", token, map[string]string{
		"sha256": hex.EncodeToString(sum[:]),
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("complete: %d %s", rec.Code, rec.Body.String())
	}
	return jsonStr(t, rec, "id")
}

func downloadArchive(t *testing.T, srv *api.Server, token, circleID string) []byte {
	t.Helper()
	rec := doGET(t, srv, "/api/v1/circles/"+circleID+"/archive/download", token)
	if rec.Code != http.StatusOK {
		t.Fatalf("download: %d %s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Content-Type") != "application/zip" {
		t.Fatalf("content-type: %s", rec.Header().Get("Content-Type"))
	}
	return rec.Body.Bytes()
}

func assertOfflineZIP(t *testing.T, data []byte) {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	var hasIndex bool
	for _, f := range zr.File {
		if f.Name == "index.html" {
			hasIndex = true
		}
		if !strings.HasSuffix(strings.ToLower(f.Name), ".html") {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		html, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatal(err)
		}
		if archive.HasExternalLinks(string(html)) {
			t.Fatalf("external links in %s", f.Name)
		}
	}
	if !hasIndex {
		t.Fatal("index.html missing")
	}
}

func eventTextRemainsDB(t *testing.T, ch *chronicle.Chronicle, circleID, needle string) (bool, error) {
	t.Helper()
	rows, err := ch.DB().QueryContext(t.Context(), `
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
		if strings.Contains(payload, needle) || strings.Contains(summary, needle) {
			return true, nil
		}
	}
	var postBody string
	err = ch.DB().QueryRowContext(t.Context(), `
		SELECT COALESCE(body, '') FROM posts WHERE circle_id = ? AND COALESCE(body, '') LIKE ?
	`, circleID, "%"+needle+"%").Scan(&postBody)
	if err == nil && postBody != "" {
		return true, nil
	}
	return false, rows.Err()
}
