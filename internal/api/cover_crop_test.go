package api_test

import (
	"net/http"
	"strings"
	"testing"
)

// Кадр обложки (4.16) сохраняется со снимком и приходит в ленту; кадр за
// краем снимка и кадр у вложения — отказ.
func TestCoverCropRoundTrip(t *testing.T) {
	srv, caps, _, _ := setupAPI(t)
	tok, _ := registerSession(t, srv, caps, "anya@example.com")
	rec := doJSON(t, srv, http.MethodPost, "/api/v1/circles", tok, map[string]any{
		"name": "Семья", "owner_name": "Аня",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("circle: %d %s", rec.Code, rec.Body.String())
	}
	circleID := jsonStr(t, rec, "id")
	post := func(media map[string]any) int {
		blob := uploadBytes(t, srv, tok, []byte("jpeg-bytes-"+strings.Repeat("x", 10)), "image/jpeg")
		media["blob_id"] = blob
		r := doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+circleID+"/posts", tok, map[string]any{
			"body": "фото", "entry_date": "2026-08-30", "media": []map[string]any{media},
		})
		return r.Code
	}
	crop := map[string]any{"x": 0.25, "y": 0, "w": 0.5, "h": 1}
	if code := post(map[string]any{"kind": "photo", "is_cover": true, "crop": crop}); code != http.StatusCreated {
		t.Fatalf("post with crop: %d", code)
	}
	feed := doGET(t, srv, "/api/v1/circles/"+circleID+"/feed", tok)
	if !strings.Contains(feed.Body.String(), `"crop":{"x":0.25,"y":0,"w":0.5,"h":1}`) {
		t.Fatalf("кадра нет в ленте: %s", feed.Body.String())
	}
	if code := post(map[string]any{"kind": "photo", "crop": map[string]any{"x": 0.8, "y": 0, "w": 0.5, "h": 1}}); code != http.StatusBadRequest {
		t.Fatalf("кадр за краем: %d", code)
	}
	if code := post(map[string]any{"kind": "attachment", "crop": crop}); code != http.StatusBadRequest {
		t.Fatalf("кадр у вложения: %d", code)
	}
}
