package api_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"testing"

	"gitea.mixdep.ru/mix/wynd/internal/poster"
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

// Ролик пришёл без кадра (Firefox на Android снять его не может) — кадр
// делает сервер, до ответа на запрос: лента сразу отдаёт его всем (план 49).
func TestServerMakesMissingVideoPoster(t *testing.T) {
	srv, caps, _, blobs := setupAPI(t)
	tok, accountID := registerSession(t, srv, caps, "poster-srv@example.com")
	rec := doJSON(t, srv, http.MethodPost, "/api/v1/circles", tok, map[string]any{
		"name": "Семья", "owner_name": "Аня",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("circle: %d %s", rec.Code, rec.Body.String())
	}
	circleID := jsonStr(t, rec, "id")

	jpeg := []byte{0xFF, 0xD8, 0xFF, 0xE0, 1, 2, 3}
	calls := 0
	srv.Posters = poster.NewFunc(blobs, func(_ context.Context, path string) ([]byte, error) {
		calls++
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		if string(data) == "broken" {
			return nil, errors.New("no frame")
		}
		return jpeg, nil
	})

	post := func(payload string, extra map[string]any) map[string]any {
		t.Helper()
		media := map[string]any{
			"blob_id": uploadBlob(t, srv, tok, []byte(payload), "video/mp4"), "kind": "video", "is_cover": true,
		}
		for k, v := range extra {
			media[k] = v
		}
		rec := doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+circleID+"/posts", tok, map[string]any{
			"body": "", "entry_date": "2026-10-07", "media": []map[string]any{media},
		})
		if rec.Code != http.StatusCreated {
			t.Fatalf("post: %d %s", rec.Code, rec.Body.String())
		}
		var out struct {
			Media []map[string]any `json:"media"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || len(out.Media) != 1 {
			t.Fatalf("post body: %v %s", err, rec.Body.String())
		}
		return out.Media[0]
	}

	made := post("video-bytes", nil)
	posterID, _ := made["video_poster_blob_id"].(string)
	if posterID == "" {
		t.Fatalf("no poster in the answer: %v", made)
	}
	b, err := blobs.LoadBlob(t.Context(), posterID)
	if err != nil || b.MimeType != "image/jpeg" || b.AccountID != accountID || b.Status != "complete" {
		t.Fatalf("poster blob: %+v %v", b, err)
	}
	rec = doJSON(t, srv, http.MethodGet, "/api/v1/blobs/"+posterID, tok, nil)
	if rec.Code != http.StatusOK || rec.Body.Len() != len(jpeg) {
		t.Fatalf("poster download: %d, %d bytes", rec.Code, rec.Body.Len())
	}

	// Кадр прислал клиент — сервер ролик не трогает.
	own := uploadBlob(t, srv, tok, []byte("client-jpeg"), "image/jpeg")
	before := calls
	if got := post("video-two", map[string]any{"video_poster_blob_id": own})["video_poster_blob_id"]; got != own || calls != before {
		t.Fatalf("client poster replaced: %v, calls %d -> %d", got, before, calls)
	}

	// С битого файла кадр не вышел: запись создана, кадра нет, и повторный
	// проход по старым роликам тот же файл не трогает.
	if got := post("broken", nil)["video_poster_blob_id"]; got != nil {
		t.Fatalf("poster from a broken file: %v", got)
	}
	before = calls
	if n := srv.Posters.Backfill(t.Context()); n != 0 || calls != before {
		t.Fatalf("backfill retried a failed video: made %d, calls %d -> %d", n, before, calls)
	}
}

// Ролики, лежавшие без кадра до этой версии, получают его проходом при старте.
func TestVideoPosterBackfill(t *testing.T) {
	srv, caps, _, blobs := setupAPI(t)
	tok, _ := registerSession(t, srv, caps, "poster-old@example.com")
	rec := doJSON(t, srv, http.MethodPost, "/api/v1/circles", tok, map[string]any{
		"name": "Семья", "owner_name": "Аня",
	})
	circleID := jsonStr(t, rec, "id")
	videoID := uploadBlob(t, srv, tok, []byte("old-video"), "video/mp4")
	rec = doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+circleID+"/posts", tok, map[string]any{
		"body": "", "entry_date": "2026-10-07",
		"media": []map[string]any{{"blob_id": videoID, "kind": "video", "is_cover": true}},
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("post: %d %s", rec.Code, rec.Body.String())
	}

	// Без ffmpeg сервер ничего не делает.
	if n := srv.Posters.Backfill(t.Context()); n != 0 {
		t.Fatalf("backfill without a maker: %d", n)
	}
	srv.Posters = poster.NewFunc(blobs, func(context.Context, string) ([]byte, error) {
		return []byte{0xFF, 0xD8, 0xFF, 0xE0}, nil
	})
	if n := srv.Posters.Backfill(t.Context()); n != 1 {
		t.Fatalf("backfill: made %d, want 1", n)
	}
	if n := srv.Posters.Backfill(t.Context()); n != 0 {
		t.Fatalf("second backfill: made %d, want 0", n)
	}
	var posterID string
	if err := blobs.DB().QueryRowContext(t.Context(),
		`SELECT COALESCE(video_poster_blob_id, '') FROM post_media WHERE blob_id = ?`, videoID).Scan(&posterID); err != nil || posterID == "" {
		t.Fatalf("poster not saved: %q %v", posterID, err)
	}
}
