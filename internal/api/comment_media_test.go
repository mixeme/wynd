package api_test

import (
	"encoding/json"
	"net/http"
	"testing"
)

// Вложение комментария: создаётся по HTTP, приходит в ленте, файл открывается
// другому участнику круга и перестаёт открываться после удаления комментария.
func TestCommentMediaOverHTTP(t *testing.T) {
	srv, caps, _, _ := setupAPI(t)
	ownerTok, _ := registerSession(t, srv, caps, "owner-cm@example.com")
	rec := doJSON(t, srv, http.MethodPost, "/api/v1/circles", ownerTok, map[string]any{
		"name": "Семья", "owner_name": "Аня",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("circle: %d %s", rec.Code, rec.Body.String())
	}
	circleID := jsonStr(t, rec, "id")
	kotTok := joinAsMember(t, srv, caps, ownerTok, circleID, "kot-cm@example.com", "Кот")
	outsiderTok, _ := registerSession(t, srv, caps, "outsider-cm@example.com")

	rec = doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+circleID+"/posts", ownerTok, map[string]any{
		"body": "яблоки", "entry_date": "2026-10-07",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("post: %d %s", rec.Code, rec.Body.String())
	}
	postID := jsonStr(t, rec, "id")
	commentsURL := "/api/v1/circles/" + circleID + "/posts/" + postID + "/comments"

	photoID := uploadBlob(t, srv, kotTok, []byte("photo-bytes"), "image/jpeg")
	voiceID := uploadBlob(t, srv, kotTok, []byte("voice-bytes"), "audio/mp4")
	foreignID := uploadBlob(t, srv, ownerTok, []byte("not-yours"), "image/jpeg")

	// Чужой блоб и видео — отказ.
	rec = doJSON(t, srv, http.MethodPost, commentsURL, kotTok, map[string]any{
		"media": []map[string]any{{"blob_id": foreignID, "kind": "photo"}},
	})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("foreign blob: %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, srv, http.MethodPost, commentsURL, kotTok, map[string]any{
		"media": []map[string]any{{"blob_id": photoID, "kind": "video"}},
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("video in a comment: %d %s", rec.Code, rec.Body.String())
	}

	// Без слов, с фото и голосовым.
	rec = doJSON(t, srv, http.MethodPost, commentsURL, kotTok, map[string]any{
		"media": []map[string]any{
			{"blob_id": photoID, "kind": "photo"},
			{"blob_id": voiceID, "kind": "attachment", "voice": true, "audio_duration_ms": 48000, "audio_peaks": []int{10, 90}},
		},
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("comment: %d %s", rec.Code, rec.Body.String())
	}
	var created struct {
		ID    string           `json:"id"`
		Media []map[string]any `json:"media"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil || len(created.Media) != 2 {
		t.Fatalf("comment body: %v %s", err, rec.Body.String())
	}

	rec = doJSON(t, srv, http.MethodGet, "/api/v1/circles/"+circleID+"/feed", ownerTok, nil)
	var feed struct {
		Posts []struct {
			Comments []struct {
				Media []map[string]any `json:"media"`
			} `json:"comments"`
		} `json:"posts"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &feed); err != nil ||
		len(feed.Posts) != 1 || len(feed.Posts[0].Comments) != 1 || len(feed.Posts[0].Comments[0].Media) != 2 {
		t.Fatalf("feed: %v %s", err, rec.Body.String())
	}
	if v := feed.Posts[0].Comments[0].Media[1]; v["voice"] != true || v["audio_duration_ms"] != float64(48000) {
		t.Fatalf("voice in feed: %v", v)
	}

	// Файл открывается участнику круга, постороннему — нет.
	if rec = doJSON(t, srv, http.MethodGet, "/api/v1/blobs/"+photoID, ownerTok, nil); rec.Code != http.StatusOK {
		t.Fatalf("member reads comment photo: %d", rec.Code)
	}
	if rec = doJSON(t, srv, http.MethodGet, "/api/v1/blobs/"+photoID, outsiderTok, nil); rec.Code == http.StatusOK {
		t.Fatalf("outsider reads comment photo: %d", rec.Code)
	}

	// Удалили комментарий — файлы ушли с ним.
	rec = doJSON(t, srv, http.MethodDelete, commentsURL+"/"+created.ID, kotTok, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("delete: %d %s", rec.Code, rec.Body.String())
	}
	if rec = doJSON(t, srv, http.MethodGet, "/api/v1/blobs/"+photoID, ownerTok, nil); rec.Code == http.StatusOK {
		t.Fatalf("photo of a deleted comment still served: %d", rec.Code)
	}
	if rec = doJSON(t, srv, http.MethodGet, "/api/v1/blobs/"+photoID, kotTok, nil); rec.Code == http.StatusOK {
		t.Fatalf("blob of a deleted comment not released: %d", rec.Code)
	}
}
