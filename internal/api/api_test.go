package api_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/api"
	"gitea.mixdep.ru/mix/wynd/internal/auth"
	"gitea.mixdep.ru/mix/wynd/internal/blob"
	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
	"gitea.mixdep.ru/mix/wynd/internal/mail"
	"gitea.mixdep.ru/mix/wynd/internal/push"
	"gitea.mixdep.ru/mix/wynd/internal/store"
	"gitea.mixdep.ru/mix/wynd/internal/version"
)

func setupFreshAPI(t *testing.T) (*api.Server, *auth.CaptureCodes, *chronicle.Chronicle, *blob.Store) {
	t.Helper()
	st, err := store.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ch, err := chronicle.New(st)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	blobs, err := blob.New(st, filepath.Join(dir, "blobs"))
	if err != nil {
		t.Fatal(err)
	}
	_ = os.MkdirAll(blobs.Dir(), 0o750)
	caps := auth.NewCaptureCodes()
	svc, err := auth.New(st, ch, caps, true)
	if err != nil {
		t.Fatal(err)
	}
	mailSvc, err := mail.New(st, true, caps)
	if err != nil {
		t.Fatal(err)
	}
	pushSvc, err := push.New(st)
	if err != nil {
		t.Fatal(err)
	}
	_ = pushSvc.EnsureKeys(t.Context())
	srv := api.NewServer(svc, ch, blobs, mailSvc, pushSvc, "bootstrap", dir, "http://127.0.0.1:7676", ":7676", true)
	// httptest.NewRequest peers from 192.0.2.1; trust it so the tests can pick
	// rate-limit buckets through X-Forwarded-For.
	srv.TrustedProxies, _ = api.ParseTrustedProxies([]string{"192.0.2.0/24"})
	return srv, caps, ch, blobs
}

func setupAPI(t *testing.T) (*api.Server, *auth.CaptureCodes, *chronicle.Chronicle, *blob.Store) {
	t.Helper()
	srv, caps, ch, blobs := setupFreshAPI(t)
	if err := srv.Auth.Bootstrap(t.Context(), auth.BootstrapInput{
		Token: "bootstrap", InstanceName: "Дом Ани", Password: "admin-pass",
		Now: time.Date(2026, 8, 30, 10, 0, 0, 0, time.UTC),
	}, "bootstrap"); err != nil {
		t.Fatal(err)
	}
	return srv, caps, ch, blobs
}

func TestInstanceEndpoint(t *testing.T) {
	srv, _, _, _ := setupAPI(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/instance", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body["name"] != "Дом Ани" {
		t.Fatalf("name: %v", body["name"])
	}
	if body["version"] != version.String() {
		t.Fatalf("version: %v want %s", body["version"], version.String())
	}
	if body["registration_mode"] != "invite" {
		t.Fatalf("mode: %v", body["registration_mode"])
	}
	if body["loopback"] != true {
		t.Fatalf("loopback: %v", body["loopback"])
	}
	if body["code_delivery"] != "log" {
		t.Fatalf("code_delivery: %v want log", body["code_delivery"])
	}
	// AGPL 13: адрес исходников отдаёт сервер, клиент его не зашивает (LIC-2).
	if body["source_url"] != version.SourceURL {
		t.Fatalf("source_url: %v want %s", body["source_url"], version.SourceURL)
	}
	comp, ok := body["compression"].(map[string]any)
	if !ok {
		t.Fatal("compression missing")
	}
	if comp["photo_max_px"].(float64) != 2048 {
		t.Fatalf("photo_max_px: %v", comp["photo_max_px"])
	}
}

func TestRegisterVerifyFlow(t *testing.T) {
	srv, caps, _, _ := setupAPI(t)
	if err := srv.Auth.SetRegistrationMode(t.Context(), auth.ModeOpen); err != nil {
		t.Fatal(err)
	}

	regBody, _ := json.Marshal(map[string]string{"email": "user@example.com"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewReader(regBody))
	req.Header.Set("X-Forwarded-For", "203.0.113.1")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("register status: %d %s", rec.Code, rec.Body.String())
	}

	verifyBody, _ := json.Marshal(map[string]string{
		"email": "user@example.com",
		"code":  caps.Last("user@example.com"),
	})
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/verify", bytes.NewReader(verifyBody))
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("verify status: %d %s", rec.Code, rec.Body.String())
	}
}

func TestAcceptInviteHTTP(t *testing.T) {
	srv, caps, ch, _ := setupAPI(t)

	circle, _, _, err := ch.CreateCircle(t.Context(), chronicle.CreateCircleInput{
		Name: "Семья", OwnerAccountID: "owner", OwnerName: "Аня",
		Now: time.Date(2026, 8, 30, 10, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	inv, err := srv.Auth.CreateInvite(t.Context(), auth.CreateInviteInput{
		CircleID: circle.ID, Kind: auth.InviteSingle, MaxUses: 1, TTL: time.Hour,
	})
	if err != nil {
		t.Fatal(err)
	}

	body, _ := json.Marshal(map[string]string{"email": "bob@example.com", "name": "Боб"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/invites/"+inv.Token+"/accept", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("accept status: %d %s", rec.Code, rec.Body.String())
	}

	verifyBody, _ := json.Marshal(map[string]string{
		"email": "bob@example.com",
		"code":  caps.Last("bob@example.com"),
	})
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/verify", bytes.NewReader(verifyBody))
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("verify status: %d %s", rec.Code, rec.Body.String())
	}
}

func registerSession(t *testing.T, srv *api.Server, caps *auth.CaptureCodes, email string) (token, accountID string) {
	t.Helper()
	if err := srv.Auth.SetRegistrationMode(t.Context(), auth.ModeOpen); err != nil {
		t.Fatal(err)
	}
	regBody, _ := json.Marshal(map[string]string{"email": email})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewReader(regBody))
	req.Header.Set("X-Forwarded-For", "203.0.113.40")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("register: %d %s", rec.Code, rec.Body.String())
	}
	verifyBody, _ := json.Marshal(map[string]string{"email": email, "code": caps.Last(email)})
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/verify", bytes.NewReader(verifyBody))
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("verify: %d %s", rec.Code, rec.Body.String())
	}
	var out map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	return out["token"].(string), out["account_id"].(string)
}

func allowCircleMultiInvites(t *testing.T, srv *api.Server, circleID, ownerTok string) {
	t.Helper()
	rec := doJSON(t, srv, http.MethodPatch, "/api/v1/circles/"+circleID, ownerTok, map[string]any{
		"invite_kind_default": "multi",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("allow multi invites: %d %s", rec.Code, rec.Body.String())
	}
}

func TestInvalidPostJSONIsBadRequest(t *testing.T) {
	srv, caps, ch, _ := setupAPI(t)
	token, accountID := registerSession(t, srv, caps, "json@example.com")
	circle, _, _, err := ch.CreateCircle(t.Context(), chronicle.CreateCircleInput{
		Name: "Семья", OwnerAccountID: accountID, OwnerName: "Аня",
		Now: time.Date(2026, 8, 30, 10, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/circles/"+circle.ID+"/posts", bytes.NewReader([]byte("{")))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status: %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestJournalContentValidation(t *testing.T) {
	srv, caps, ch, _ := setupAPI(t)
	token, accountID := registerSession(t, srv, caps, "validate@example.com")
	circle, _, _, err := ch.CreateCircle(t.Context(), chronicle.CreateCircleInput{
		Name: "Семья", OwnerAccountID: accountID, OwnerName: "Аня",
		Now: time.Date(2026, 8, 30, 10, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	postURL := "/api/v1/circles/" + circle.ID + "/posts"

	whitespacePost, _ := json.Marshal(map[string]any{"body": "   ", "media": []any{}, "entry_date": "2026-08-30"})
	req := httptest.NewRequest(http.MethodPost, postURL, bytes.NewReader(whitespacePost))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("whitespace post: %d %s", rec.Code, rec.Body.String())
	}

	badDate, _ := json.Marshal(map[string]any{"body": "текст", "entry_date": "2026-02-30"})
	req = httptest.NewRequest(http.MethodPost, postURL, bytes.NewReader(badDate))
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid entry_date: %d %s", rec.Code, rec.Body.String())
	}

	okBody, _ := json.Marshal(map[string]any{"body": "запись", "entry_date": "2026-08-30"})
	req = httptest.NewRequest(http.MethodPost, postURL, bytes.NewReader(okBody))
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create post: %d %s", rec.Code, rec.Body.String())
	}
	var post map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&post); err != nil {
		t.Fatal(err)
	}
	postID := post["id"].(string)

	whitespaceComment, _ := json.Marshal(map[string]string{"body": "   "})
	req = httptest.NewRequest(http.MethodPost, "/api/v1/circles/"+circle.ID+"/posts/"+postID+"/comments", bytes.NewReader(whitespaceComment))
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("whitespace comment: %d %s", rec.Code, rec.Body.String())
	}
}

func TestMediaOnlyPostAndTextWhenQuotaFull(t *testing.T) {
	srv, caps, ch, blobs := setupAPI(t)
	token, accountID := registerSession(t, srv, caps, "media@example.com")
	circle, _, _, err := ch.CreateCircle(t.Context(), chronicle.CreateCircleInput{
		Name: "Семья", OwnerAccountID: accountID, OwnerName: "Аня",
		Now: time.Date(2026, 8, 30, 10, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}

	body, _ := json.Marshal(map[string]any{
		"body": "", "entry_date": "2026-08-30",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/circles/"+circle.ID+"/posts", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("empty post without media: %d %s", rec.Code, rec.Body.String())
	}

	payload := []byte("photo-bytes")
	upBody, _ := json.Marshal(map[string]any{"expected_size": len(payload), "mime_type": "image/jpeg"})
	req = httptest.NewRequest(http.MethodPost, "/api/v1/uploads", bytes.NewReader(upBody))
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("upload session: %d %s", rec.Code, rec.Body.String())
	}
	var sess map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&sess); err != nil {
		t.Fatal(err)
	}
	sessionID := sess["id"].(string)
	req = httptest.NewRequest(http.MethodPut, "/api/v1/uploads/"+sessionID, bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Upload-Offset", "0")
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("chunk: %d %s", rec.Code, rec.Body.String())
	}
	sum := sha256.Sum256(payload)
	done, _ := json.Marshal(map[string]string{"sha256": hex.EncodeToString(sum[:])})
	req = httptest.NewRequest(http.MethodPost, "/api/v1/uploads/"+sessionID+"/complete", bytes.NewReader(done))
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("complete: %d %s", rec.Code, rec.Body.String())
	}
	var blobInfo map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&blobInfo); err != nil {
		t.Fatal(err)
	}
	blobID := blobInfo["id"].(string)

	postBody, _ := json.Marshal(map[string]any{
		"body": "", "entry_date": "2026-08-30",
		"media": []map[string]any{{"blob_id": blobID, "kind": "photo", "is_cover": true}},
	})
	req = httptest.NewRequest(http.MethodPost, "/api/v1/circles/"+circle.ID+"/posts", bytes.NewReader(postBody))
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("media-only post: %d %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/blobs/"+blobID, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("serve blob: %d %s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Content-Disposition")[:10] != "attachment" {
		t.Fatalf("disposition: %s", rec.Header().Get("Content-Disposition"))
	}
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("missing nosniff")
	}

	if _, err := blobs.DB().ExecContext(t.Context(), `
		UPDATE instance_settings SET storage_quota_bytes = 1, storage_quota_disk_percent = NULL WHERE id = 1
	`); err != nil {
		t.Fatal(err)
	}
	textBody, _ := json.Marshal(map[string]any{"body": "только текст", "entry_date": "2026-08-30"})
	req = httptest.NewRequest(http.MethodPost, "/api/v1/circles/"+circle.ID+"/posts", bytes.NewReader(textBody))
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("text post at quota: %d %s", rec.Code, rec.Body.String())
	}
	blocked, _ := json.Marshal(map[string]any{"expected_size": 20, "mime_type": "image/jpeg"})
	req = httptest.NewRequest(http.MethodPost, "/api/v1/uploads", bytes.NewReader(blocked))
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusInsufficientStorage {
		t.Fatalf("upload at quota: %d %s", rec.Code, rec.Body.String())
	}

	syncSSE(t, srv, token, 0)
}

func TestHTMLBlobNotExecuted(t *testing.T) {
	srv, caps, _, _ := setupAPI(t)
	token, _ := registerSession(t, srv, caps, "html@example.com")
	payload := []byte("<html><script>alert(1)</script></html>")
	upBody, _ := json.Marshal(map[string]any{"expected_size": len(payload), "mime_type": "text/html"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/uploads", bytes.NewReader(upBody))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("session: %d %s", rec.Code, rec.Body.String())
	}
	var sess map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&sess); err != nil {
		t.Fatal(err)
	}
	sessionID := sess["id"].(string)
	req = httptest.NewRequest(http.MethodPut, "/api/v1/uploads/"+sessionID, bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Upload-Offset", "0")
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("chunk: %d %s", rec.Code, rec.Body.String())
	}
	sum := sha256.Sum256(payload)
	done, _ := json.Marshal(map[string]string{"sha256": hex.EncodeToString(sum[:])})
	req = httptest.NewRequest(http.MethodPost, "/api/v1/uploads/"+sessionID+"/complete", bytes.NewReader(done))
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("complete: %d %s", rec.Code, rec.Body.String())
	}
	var blobInfo map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&blobInfo); err != nil {
		t.Fatal(err)
	}
	req = httptest.NewRequest(http.MethodGet, "/api/v1/blobs/"+blobInfo["id"].(string), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("serve: %d %s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Content-Type") != "application/octet-stream" {
		t.Fatalf("content-type: %s", rec.Header().Get("Content-Type"))
	}
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("missing nosniff")
	}
}
