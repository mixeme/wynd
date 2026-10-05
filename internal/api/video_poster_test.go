package api_test

import (
	"net/http"
	"testing"
)

func TestVideoPosterRejected(t *testing.T) {
	srv, caps, _, _ := setupAPI(t)
	ownerTok, _ := registerSession(t, srv, caps, "owner-poster@example.com")
	otherTok, _ := registerSession(t, srv, caps, "other-poster@example.com")
	rec := doJSON(t, srv, http.MethodPost, "/api/v1/circles", ownerTok, map[string]any{
		"name": "Семья", "owner_name": "Аня",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("circle: %d %s", rec.Code, rec.Body.String())
	}
	circleID := jsonStr(t, rec, "id")
	videoID := uploadBlob(t, srv, ownerTok, []byte("video-bytes"), "video/mp4")
	pngID := uploadBlob(t, srv, ownerTok, []byte("png-bytes"), "image/png")
	foreignID := uploadBlob(t, srv, otherTok, []byte("jpeg-bytes"), "image/jpeg")

	rec = doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+circleID+"/posts", ownerTok, map[string]any{
		"body": "", "entry_date": "2026-10-04",
		"media": []map[string]any{{
			"blob_id": videoID, "kind": "video", "is_cover": true,
			"video_poster_blob_id": pngID,
		}},
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("png poster: %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+circleID+"/posts", ownerTok, map[string]any{
		"body": "", "entry_date": "2026-10-04",
		"media": []map[string]any{{
			"blob_id": videoID, "kind": "video", "is_cover": true,
			"video_poster_blob_id": foreignID,
		}},
	})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("foreign poster: %d %s", rec.Code, rec.Body.String())
	}
}
