package api_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
)

func TestPatchPostMedia(t *testing.T) {
	srv, caps, ch, blobs := setupAPI(t)
	token, accountID := registerSession(t, srv, caps, "patch-media@example.com")
	circle, _, _, err := ch.CreateCircle(t.Context(), chronicle.CreateCircleInput{
		Name: "Семья", OwnerAccountID: accountID, OwnerName: "Аня",
		Now: time.Date(2026, 8, 30, 10, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	circleID := circle.ID
	entryDate := "2026-08-30"
	blobA := uploadBytes(t, srv, token, []byte("photo-a"), "image/jpeg")
	blobB := uploadBytes(t, srv, token, []byte("photo-b"), "image/jpeg")

	rec := doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+circleID+"/posts", token, map[string]any{
		"body": "с фото", "entry_date": entryDate,
		"media": []map[string]any{{"blob_id": blobA, "kind": "photo", "is_cover": true}},
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create post: %d %s", rec.Code, rec.Body.String())
	}
	postID := jsonStr(t, rec, "id")

	rec = doJSON(t, srv, http.MethodPut, "/api/v1/circles/"+circleID+"/days/"+entryDate+"/cover", token, map[string]string{
		"post_id": postID, "blob_id": blobA,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("set day cover: %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, srv, http.MethodPatch, "/api/v1/circles/"+circleID+"/posts/"+postID, token, map[string]string{
		"body": "только текст",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("patch without media field: %d %s", rec.Code, rec.Body.String())
	}
	rec = doGET(t, srv, "/api/v1/circles/"+circleID+"/feed", token)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), blobA) {
		t.Fatalf("media unchanged when field omitted: %s", rec.Body.String())
	}

	rec = doJSON(t, srv, http.MethodPatch, "/api/v1/circles/"+circleID+"/posts/"+postID, token, map[string]any{
		"body": "", "entry_date": entryDate, "media": []any{},
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("empty body and media: %d %s", rec.Code, rec.Body.String())
	}

	if _, err := blobs.DB().ExecContext(t.Context(), `
		UPDATE instance_settings SET storage_quota_bytes = 1, storage_quota_disk_percent = NULL WHERE id = 1
	`); err != nil {
		t.Fatal(err)
	}
	rec = doJSON(t, srv, http.MethodPatch, "/api/v1/circles/"+circleID+"/posts/"+postID, token, map[string]any{
		"body": "добавить", "entry_date": entryDate,
		"media": []map[string]any{
			{"blob_id": blobA, "kind": "photo", "is_cover": true},
			{"blob_id": blobB, "kind": "photo", "is_cover": false},
		},
	})
	if rec.Code != http.StatusInsufficientStorage {
		t.Fatalf("quota on newly added blob: %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, srv, http.MethodPatch, "/api/v1/circles/"+circleID+"/posts/"+postID, token, map[string]any{
		"body": "оставить одно", "entry_date": entryDate,
		"media": []map[string]any{{"blob_id": blobA, "kind": "photo", "is_cover": true}},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("patch keeping existing blob at quota: %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, srv, http.MethodPatch, "/api/v1/circles/"+circleID+"/posts/"+postID, token, map[string]any{
		"body": "без фото", "entry_date": entryDate, "media": []any{},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("text-only patch with empty media: %d %s", rec.Code, rec.Body.String())
	}
	rec = doGET(t, srv, "/api/v1/circles/"+circleID+"/days", token)
	if strings.Contains(rec.Body.String(), blobA) {
		t.Fatalf("day cover should clear after cover blob removed: %s", rec.Body.String())
	}
}
