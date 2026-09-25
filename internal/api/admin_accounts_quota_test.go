package api_test

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/auth"
	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
)

func TestAdminDeleteAccountOwnerConflictAndBlobsRemain(t *testing.T) {
	srv, caps, ch, blobs := setupAPI(t)
	ownerTok, ownerID := registerSession(t, srv, caps, "owner@example.com")
	memberTok, memberID := registerSession(t, srv, caps, "member@example.com")

	circle, _, _, err := ch.CreateCircle(t.Context(), chronicle.CreateCircleInput{
		Name: "Семья", OwnerAccountID: ownerID, OwnerName: "Владелец",
		Now: time.Date(2026, 8, 30, 10, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := ch.Join(t.Context(), chronicle.JoinInput{
		CircleID: circle.ID, AccountID: memberID, Name: "Участник",
		Now: time.Date(2026, 8, 30, 11, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatal(err)
	}

	blobID := uploadBytes(t, srv, memberTok, []byte("blob-payload-for-delete-test"), "image/jpeg")
	rec := doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+circle.ID+"/posts", memberTok, map[string]any{
		"body": "фото", "entry_date": "2026-08-30",
		"media": []map[string]any{{"blob_id": blobID, "kind": "photo", "is_cover": true}},
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("post: %d %s", rec.Code, rec.Body.String())
	}

	admin := adminToken(t, srv)
	rec = doJSON(t, srv, http.MethodDelete, "/api/v1/admin/accounts/"+ownerID, admin, nil)
	if rec.Code != http.StatusConflict {
		t.Fatalf("owner delete: %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, srv, http.MethodDelete, "/api/v1/admin/accounts/"+memberID, admin, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("member delete: %d %s", rec.Code, rec.Body.String())
	}

	var orphanIdentities int
	if err := ch.DB().QueryRowContext(t.Context(), `
		SELECT COUNT(*) FROM identities WHERE account_id IS NULL
	`).Scan(&orphanIdentities); err != nil {
		t.Fatal(err)
	}
	if orphanIdentities < 1 {
		t.Fatalf("identities should be orphaned, count=%d", orphanIdentities)
	}

	var postCount int
	if err := ch.DB().QueryRowContext(t.Context(), `
		SELECT COUNT(*) FROM posts WHERE circle_id = ? AND deleted = 0
	`, circle.ID).Scan(&postCount); err != nil {
		t.Fatal(err)
	}
	if postCount != 1 {
		t.Fatalf("posts should remain, count=%d", postCount)
	}

	rec = doGET(t, srv, "/api/v1/admin/accounts/"+memberID, admin)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("deleted get: %d %s", rec.Code, rec.Body.String())
	}

	var blobCount int
	if err := blobs.DB().QueryRowContext(t.Context(), `
		SELECT COUNT(*) FROM blobs WHERE id = ? AND status = 'complete'
	`, blobID).Scan(&blobCount); err != nil {
		t.Fatal(err)
	}
	if blobCount != 1 {
		t.Fatalf("blob should remain after account delete, count=%d", blobCount)
	}

	rec = doGET(t, srv, "/api/v1/circles", memberTok)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("deleted session: %d %s", rec.Code, rec.Body.String())
	}
	_ = ownerTok
}

func TestAdminAccountDetailEmptyCirclesAndUnknownQuota(t *testing.T) {
	srv, caps, _, _ := setupAPI(t)
	_, ghostID := registerSession(t, srv, caps, "ghost@example.com")
	admin := adminToken(t, srv)

	rec := doGET(t, srv, "/api/v1/admin/accounts/"+ghostID, admin)
	if rec.Code != http.StatusOK {
		t.Fatalf("ghost get: %d %s", rec.Code, rec.Body.String())
	}
	var raw map[string]json.RawMessage
	if err := json.NewDecoder(rec.Body).Decode(&raw); err != nil {
		t.Fatal(err)
	}
	if string(raw["circles"]) == "null" || len(raw["circles"]) == 0 {
		t.Fatalf("circles must be [], got %s", raw["circles"])
	}
	var circles []any
	if err := json.Unmarshal(raw["circles"], &circles); err != nil {
		t.Fatal(err)
	}
	if len(circles) != 0 {
		t.Fatalf("ghost circles: %+v", circles)
	}

	rec = doJSON(t, srv, http.MethodPut, "/api/v1/admin/circles/no-such/quota", admin, map[string]any{
		"custom": false,
	})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unknown circle quota: %d %s", rec.Code, rec.Body.String())
	}
}

func TestAdminDefaultQuotaDoesNotTouchCircles(t *testing.T) {
	srv, caps, ch, blobs := setupAPI(t)
	token, ownerID := registerSession(t, srv, caps, "owner@example.com")
	circle, _, _, err := ch.CreateCircle(t.Context(), chronicle.CreateCircleInput{
		Name: "Семья", OwnerAccountID: ownerID, OwnerName: "Владелец",
		Now: time.Date(2026, 8, 30, 10, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	admin := adminToken(t, srv)
	custom := int64(500_000_000)
	rec := doJSON(t, srv, http.MethodPut, "/api/v1/admin/circles/"+circle.ID+"/quota", admin, map[string]any{
		"custom": true, "quota_bytes": custom,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("set circle quota: %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, srv, http.MethodPut, "/api/v1/admin/storage/default_quota", admin, map[string]any{
		"default_circle_quota_bytes": 123,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("set default quota: %d %s", rec.Code, rec.Body.String())
	}

	var quotaCustom int
	var quotaBytes sql.NullInt64
	if err := blobs.DB().QueryRowContext(t.Context(), `
		SELECT quota_custom, quota_bytes FROM circles WHERE id = ?
	`, circle.ID).Scan(&quotaCustom, &quotaBytes); err != nil {
		t.Fatal(err)
	}
	if quotaCustom != 1 || !quotaBytes.Valid || quotaBytes.Int64 != custom {
		t.Fatalf("circle quota changed: custom=%d bytes=%v", quotaCustom, quotaBytes)
	}
	_ = token
}

func TestAdminCircleQuotaModesAndPendingApproval(t *testing.T) {
	srv, caps, ch, blobs := setupAPI(t)
	token, ownerID := registerSession(t, srv, caps, "owner@example.com")
	circle, _, _, err := ch.CreateCircle(t.Context(), chronicle.CreateCircleInput{
		Name: "Семья", OwnerAccountID: ownerID, OwnerName: "Владелец",
		Now: time.Date(2026, 8, 30, 10, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	admin := adminToken(t, srv)

	rec := doJSON(t, srv, http.MethodPut, "/api/v1/admin/circles/"+circle.ID+"/quota", admin, map[string]any{
		"custom": true, "quota_bytes": 777,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("custom quota: %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+circle.ID+"/quota_requests", token, map[string]any{
		"requested_bytes": 999,
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("quota request: %d %s", rec.Code, rec.Body.String())
	}
	var reqOut map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&reqOut); err != nil {
		t.Fatal(err)
	}
	reqID := reqOut["id"]

	rec = doJSON(t, srv, http.MethodPut, "/api/v1/admin/circles/"+circle.ID+"/quota", admin, map[string]any{
		"custom": false,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("reset custom: %d %s", rec.Code, rec.Body.String())
	}
	var quotaCustom int
	if err := blobs.DB().QueryRowContext(t.Context(), `
		SELECT quota_custom FROM circles WHERE id = ?
	`, circle.ID).Scan(&quotaCustom); err != nil {
		t.Fatal(err)
	}
	if quotaCustom != 0 {
		t.Fatalf("quota_custom: %d", quotaCustom)
	}

	var status, resolved sql.NullString
	if err := blobs.DB().QueryRowContext(t.Context(), `
		SELECT status, resolved_at FROM quota_requests WHERE id = ?
	`, reqID).Scan(&status, &resolved); err != nil {
		t.Fatal(err)
	}
	if status.String != "approved" || !resolved.Valid || resolved.String == "" {
		t.Fatalf("pending request: status=%q resolved=%v", status.String, resolved.Valid)
	}

	rec = doJSON(t, srv, http.MethodPut, "/api/v1/admin/circles/"+circle.ID+"/quota", admin, map[string]any{
		"custom": true,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("no-quota custom: %d %s", rec.Code, rec.Body.String())
	}
	var quotaBytes sql.NullInt64
	if err := blobs.DB().QueryRowContext(t.Context(), `
		SELECT quota_custom, quota_bytes FROM circles WHERE id = ?
	`, circle.ID).Scan(&quotaCustom, &quotaBytes); err != nil {
		t.Fatal(err)
	}
	if quotaCustom != 1 || quotaBytes.Valid {
		t.Fatalf("expected custom without bytes: custom=%d bytes=%v", quotaCustom, quotaBytes)
	}
}

func TestAdminInviteTTLSec(t *testing.T) {
	srv, _, _, _ := setupAPI(t)
	admin := adminToken(t, srv)
	before := time.Now().UTC()
	rec := doJSON(t, srv, http.MethodPost, "/api/v1/admin/invites", admin, map[string]any{
		"kind": "single", "max_uses": 1, "ttl_sec": 3600,
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("invite: %d %s", rec.Code, rec.Body.String())
	}
	var out map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	expires, err := time.Parse(time.RFC3339, out["expires_at"].(string))
	if err != nil {
		t.Fatal(err)
	}
	delta := expires.Sub(before)
	if delta < 59*time.Minute || delta > 61*time.Minute {
		t.Fatalf("ttl_sec expiry delta: %v", delta)
	}
}

func TestAdminDeleteSentinelNotFound(t *testing.T) {
	srv, _, _, _ := setupAPI(t)
	admin := adminToken(t, srv)
	var sentinelID string
	if err := srv.Auth.DB().QueryRowContext(t.Context(), `
		SELECT id FROM accounts WHERE email = ?
	`, auth.AdminSentinelEmail).Scan(&sentinelID); err != nil {
		t.Fatal(err)
	}
	rec := doJSON(t, srv, http.MethodDelete, "/api/v1/admin/accounts/"+sentinelID, admin, nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("sentinel delete: %d %s", rec.Code, rec.Body.String())
	}
}

func TestAdminDeleteAllowsSameEmailReRegister(t *testing.T) {
	srv, caps, ch, _ := setupAPI(t)
	email := "return@example.com"
	_, ownerID := registerSession(t, srv, caps, "owner@example.com")
	_, memberID := registerSession(t, srv, caps, email)
	circle, _, _, err := ch.CreateCircle(t.Context(), chronicle.CreateCircleInput{
		Name: "Семья", OwnerAccountID: ownerID, OwnerName: "Владелец",
		Now: time.Date(2026, 8, 30, 10, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := ch.Join(t.Context(), chronicle.JoinInput{
		CircleID: circle.ID, AccountID: memberID, Name: "Гость",
		Now: time.Date(2026, 8, 30, 11, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatal(err)
	}

	admin := adminToken(t, srv)
	rec := doJSON(t, srv, http.MethodDelete, "/api/v1/admin/accounts/"+memberID, admin, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("member delete: %d %s", rec.Code, rec.Body.String())
	}

	newTok, newID := registerSession(t, srv, caps, email)
	if newID == memberID {
		t.Fatal("re-register should create a new account id")
	}
	rec = doGET(t, srv, "/api/v1/circles", newTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("re-registered session: %d %s", rec.Code, rec.Body.String())
	}
}

func TestAdminListAccountsActiveCircleCount(t *testing.T) {
	srv, caps, ch, _ := setupAPI(t)
	_, ownerID := registerSession(t, srv, caps, "owner@example.com")
	memberTok, memberID := registerSession(t, srv, caps, "member@example.com")
	circle, _, _, err := ch.CreateCircle(t.Context(), chronicle.CreateCircleInput{
		Name: "Семья", OwnerAccountID: ownerID, OwnerName: "Владелец",
		Now: time.Date(2026, 8, 30, 10, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := ch.Join(t.Context(), chronicle.JoinInput{
		CircleID: circle.ID, AccountID: memberID, Name: "Участник",
		Now: time.Date(2026, 8, 30, 11, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatal(err)
	}
	if err := ch.Leave(t.Context(), circle.ID, memberID, time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}

	admin := adminToken(t, srv)
	rec := doGET(t, srv, "/api/v1/admin/accounts", admin)
	if rec.Code != http.StatusOK {
		t.Fatalf("list accounts: %d %s", rec.Code, rec.Body.String())
	}
	var list struct {
		Accounts []struct {
			ID          string `json:"id"`
			Email       string `json:"email"`
			CircleCount int    `json:"circle_count"`
		} `json:"accounts"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&list); err != nil {
		t.Fatal(err)
	}
	var ownerCount, memberCount int
	var sawOwner, sawMember bool
	for _, acc := range list.Accounts {
		switch acc.Email {
		case "owner@example.com":
			sawOwner = true
			ownerCount = acc.CircleCount
		case "member@example.com":
			sawMember = true
			memberCount = acc.CircleCount
		}
	}
	if !sawOwner || ownerCount != 1 {
		t.Fatalf("owner circle_count: saw=%v count=%d", sawOwner, ownerCount)
	}
	if !sawMember || memberCount != 0 {
		t.Fatalf("left member circle_count: saw=%v count=%d", sawMember, memberCount)
	}
	_ = memberTok
}

func TestAdminAccountDetailJoinedAt(t *testing.T) {
	srv, caps, ch, _ := setupAPI(t)
	_, ownerID := registerSession(t, srv, caps, "owner@example.com")
	joinAt := time.Date(2026, 3, 12, 10, 0, 0, 0, time.UTC)
	circle, _, _, err := ch.CreateCircle(t.Context(), chronicle.CreateCircleInput{
		Name: "Семья", OwnerAccountID: ownerID, OwnerName: "Владелец",
		Now: joinAt,
	})
	if err != nil {
		t.Fatal(err)
	}
	admin := adminToken(t, srv)
	rec := doGET(t, srv, "/api/v1/admin/accounts/"+ownerID, admin)
	if rec.Code != http.StatusOK {
		t.Fatalf("account detail: %d %s", rec.Code, rec.Body.String())
	}
	var detail struct {
		Circles []struct {
			ID       string `json:"id"`
			JoinedAt string `json:"joined_at"`
		} `json:"circles"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&detail); err != nil {
		t.Fatal(err)
	}
	if len(detail.Circles) != 1 || detail.Circles[0].ID != circle.ID || detail.Circles[0].JoinedAt == "" {
		t.Fatalf("joined_at missing: %+v", detail.Circles)
	}
}
