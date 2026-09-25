package api_test

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

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
