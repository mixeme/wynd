package api_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"gitea.mixdep.ru/mix/wynd/internal/api"
)

// feedPosts возвращает ленту круга глазами владельца токена.
func feedPosts(t *testing.T, srv *api.Server, token, circleID string) []map[string]any {
	t.Helper()
	rec := doGET(t, srv, "/api/v1/circles/"+circleID+"/feed", token)
	if rec.Code != http.StatusOK {
		t.Fatalf("feed: %d %s", rec.Code, rec.Body.String())
	}
	var out struct {
		Posts []map[string]any `json:"posts"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out.Posts
}

func firstPost(t *testing.T, srv *api.Server, token, circleID string) map[string]any {
	t.Helper()
	posts := feedPosts(t, srv, token, circleID)
	if len(posts) != 1 {
		t.Fatalf("в ленте %d записей, ожидалась одна", len(posts))
	}
	return posts[0]
}

// Инвариант: комментарий правит и удаляет только его автор. Владелец круга
// — не исключение: у сказанного есть автор, и стереть чужое нельзя даже
// хозяину.
func TestCommentEditDeleteOnlyByAuthor(t *testing.T) {
	srv, caps, _, _ := setupAPI(t)
	ownerTok, memberTok, circleID, _ := circleWithMember(t, srv, caps)

	rec := doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+circleID+"/posts", ownerTok, map[string]any{
		"body": "запись", "entry_date": "2026-08-30",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("запись: %d %s", rec.Code, rec.Body.String())
	}
	postID := jsonStr(t, rec, "id")

	rec = doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+circleID+"/posts/"+postID+"/comments", memberTok, map[string]any{
		"body": "комментарий Боба",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("комментарий: %d %s", rec.Code, rec.Body.String())
	}
	commentID := jsonStr(t, rec, "id")
	base := "/api/v1/circles/" + circleID + "/posts/" + postID + "/comments/" + commentID

	// Владелец круга не автор — и правки, и удаления ему отказано.
	if rec := doJSON(t, srv, http.MethodPatch, base, ownerTok, map[string]any{"body": "подменили"}); rec.Code != http.StatusForbidden {
		t.Fatalf("правка чужого комментария: %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, srv, http.MethodDelete, base, ownerTok, nil); rec.Code != http.StatusForbidden {
		t.Fatalf("удаление чужого комментария: %d %s", rec.Code, rec.Body.String())
	}
	comments := firstPost(t, srv, ownerTok, circleID)["comments"].([]any)
	if len(comments) != 1 || comments[0].(map[string]any)["body"] != "комментарий Боба" {
		t.Fatalf("комментарий изменён отказанными запросами: %+v", comments)
	}

	// Автор правит и удаляет.
	if rec := doJSON(t, srv, http.MethodPatch, base, memberTok, map[string]any{"body": "уточнил"}); rec.Code != http.StatusOK {
		t.Fatalf("правка своего комментария: %d %s", rec.Code, rec.Body.String())
	}
	comments = firstPost(t, srv, ownerTok, circleID)["comments"].([]any)
	if comments[0].(map[string]any)["body"] != "уточнил" {
		t.Fatalf("правка не применилась: %+v", comments)
	}
	if rec := doJSON(t, srv, http.MethodDelete, base, memberTok, nil); rec.Code != http.StatusOK {
		t.Fatalf("удаление своего комментария: %d %s", rec.Code, rec.Body.String())
	}
	if _, ok := firstPost(t, srv, ownerTok, circleID)["comments"]; ok {
		t.Fatal("комментарий остался в ленте после удаления")
	}
}

// Инвариант: реакция на запись у участника одна. Повторный PUT заменяет
// её, а не добавляет вторую; DELETE снимает свою и не трогает чужую.
func TestReactionSetReplaceDelete(t *testing.T) {
	srv, caps, _, _ := setupAPI(t)
	ownerTok, memberTok, circleID, _ := circleWithMember(t, srv, caps)

	rec := doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+circleID+"/posts", ownerTok, map[string]any{
		"body": "запись", "entry_date": "2026-08-30",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("запись: %d %s", rec.Code, rec.Body.String())
	}
	postID := jsonStr(t, rec, "id")
	path := "/api/v1/circles/" + circleID + "/posts/" + postID + "/reactions"

	reactions := func(token string) []any {
		t.Helper()
		post := firstPost(t, srv, token, circleID)
		raw, ok := post["reactions"]
		if !ok {
			return nil
		}
		return raw.([]any)
	}

	if rec := doJSON(t, srv, http.MethodPut, path, memberTok, map[string]any{"emoji": "heart"}); rec.Code != http.StatusOK {
		t.Fatalf("реакция: %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, srv, http.MethodPut, path, memberTok, map[string]any{"emoji": "laugh"}); rec.Code != http.StatusOK {
		t.Fatalf("замена реакции: %d %s", rec.Code, rec.Body.String())
	}
	list := reactions(ownerTok)
	if len(list) != 1 || list[0].(map[string]any)["emoji"] != "laugh" {
		t.Fatalf("повтор добавил вторую реакцию вместо замены: %+v", list)
	}

	// Неизвестный ключ не принимается и прежнюю реакцию не сносит.
	if rec := doJSON(t, srv, http.MethodPut, path, memberTok, map[string]any{"emoji": "🔥"}); rec.Code != http.StatusBadRequest {
		t.Fatalf("произвольный эмодзи: %d %s", rec.Code, rec.Body.String())
	}
	if list := reactions(ownerTok); len(list) != 1 || list[0].(map[string]any)["emoji"] != "laugh" {
		t.Fatalf("отказ снял реакцию: %+v", list)
	}

	// Вторая реакция — от другого участника, обе живут рядом.
	if rec := doJSON(t, srv, http.MethodPut, path, ownerTok, map[string]any{"emoji": "heart"}); rec.Code != http.StatusOK {
		t.Fatalf("реакция владельца: %d %s", rec.Code, rec.Body.String())
	}
	if list := reactions(ownerTok); len(list) != 2 {
		t.Fatalf("реакции двух участников: %+v", list)
	}

	// DELETE снимает только свою.
	if rec := doJSON(t, srv, http.MethodDelete, path, memberTok, nil); rec.Code != http.StatusOK {
		t.Fatalf("снятие реакции: %d %s", rec.Code, rec.Body.String())
	}
	list = reactions(ownerTok)
	if len(list) != 1 || list[0].(map[string]any)["emoji"] != "heart" {
		t.Fatalf("снята чужая реакция: %+v", list)
	}
	// Снимать нечего — отказ, а не молчаливое «ок».
	if rec := doJSON(t, srv, http.MethodDelete, path, memberTok, nil); rec.Code != http.StatusNotFound {
		t.Fatalf("повторное снятие: %d %s", rec.Code, rec.Body.String())
	}
}
