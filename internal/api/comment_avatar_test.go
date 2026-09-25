package api_test

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
)

func TestCommentResponseIncludesAuthorAvatar(t *testing.T) {
	srv, caps, ch, _ := setupAPI(t)
	token, accountID := registerSession(t, srv, caps, "comment-avatar@example.com")
	circle, _, _, err := ch.CreateCircle(t.Context(), chronicle.CreateCircleInput{
		Name: "Семья", OwnerAccountID: accountID, OwnerName: "Аня",
		Now: time.Date(2026, 8, 30, 10, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	face := uploadBytes(t, srv, token, []byte("face"), "image/jpeg")
	rec := doJSON(t, srv, http.MethodPut, "/api/v1/circles/"+circle.ID+"/identity", token, map[string]string{
		"avatar_blob_id": face,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("set avatar: %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+circle.ID+"/posts", token, map[string]any{
		"body": "привет", "entry_date": "2026-08-30",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create post: %d %s", rec.Code, rec.Body.String())
	}
	postID := jsonStr(t, rec, "id")

	rec = doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+circle.ID+"/posts/"+postID+"/comments", token, map[string]string{
		"body": "ответ",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create comment: %d %s", rec.Code, rec.Body.String())
	}
	if jsonStr(t, rec, "author_avatar_blob_id") != face {
		t.Fatalf("create comment avatar: %s", rec.Body.String())
	}

	rec = doGET(t, srv, "/api/v1/circles/"+circle.ID+"/feed", token)
	if rec.Code != http.StatusOK {
		t.Fatalf("feed: %d %s", rec.Code, rec.Body.String())
	}
	var snap struct {
		Posts []struct {
			Comments []struct {
				AuthorAvatarBlobID string `json:"author_avatar_blob_id"`
			} `json:"comments"`
		} `json:"posts"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &snap); err != nil {
		t.Fatal(err)
	}
	if len(snap.Posts) != 1 || len(snap.Posts[0].Comments) != 1 {
		t.Fatalf("feed shape: %s", rec.Body.String())
	}
	if snap.Posts[0].Comments[0].AuthorAvatarBlobID != face {
		t.Fatalf("feed comment avatar: %s", rec.Body.String())
	}
}
