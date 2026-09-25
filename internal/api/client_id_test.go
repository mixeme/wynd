package api_test

import (
	"encoding/json"
	"net/http"
	"testing"
)

// Инвариант (CLI-2): повтор POST с тем же client_id не создаёт дубль, а
// возвращает уже созданную сущность. Так офлайн-очередь переживает потерю
// ответа на принятый запрос.
func TestCreatePostAndCommentAreIdempotentByClientID(t *testing.T) {
	srv, caps, _, _ := setupAPI(t)
	token, _ := registerSession(t, srv, caps, "dup@example.com")
	rec := doJSON(t, srv, http.MethodPost, "/api/v1/circles", token, map[string]any{
		"name": "Семья", "owner_name": "Аня",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create circle: %d %s", rec.Code, rec.Body.String())
	}
	circleID := jsonStr(t, rec, "id")

	post := map[string]any{
		"body": "одна запись", "entry_date": "2026-08-30", "client_id": "cid-post-1",
	}
	first := doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+circleID+"/posts", token, post)
	if first.Code != http.StatusCreated {
		t.Fatalf("первая запись: %d %s", first.Code, first.Body.String())
	}
	postID := jsonStr(t, first, "id")

	second := doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+circleID+"/posts", token, post)
	if second.Code != http.StatusCreated {
		t.Fatalf("повтор записи: %d %s", second.Code, second.Body.String())
	}
	if got := jsonStr(t, second, "id"); got != postID {
		t.Fatalf("повтор создал вторую запись: %s != %s", got, postID)
	}

	// Состояние: в ленте одна запись.
	feed := doGET(t, srv, "/api/v1/circles/"+circleID+"/feed", token)
	if feed.Code != http.StatusOK {
		t.Fatalf("feed: %d %s", feed.Code, feed.Body.String())
	}
	var body map[string]any
	if err := json.NewDecoder(feed.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	posts, _ := body["posts"].([]any)
	if len(posts) != 1 {
		t.Fatalf("в ленте %d записей, ожидалась одна", len(posts))
	}

	comment := map[string]any{"body": "один комментарий", "client_id": "cid-comment-1"}
	path := "/api/v1/circles/" + circleID + "/posts/" + postID + "/comments"
	c1 := doJSON(t, srv, http.MethodPost, path, token, comment)
	if c1.Code != http.StatusCreated {
		t.Fatalf("первый комментарий: %d %s", c1.Code, c1.Body.String())
	}
	c2 := doJSON(t, srv, http.MethodPost, path, token, comment)
	if c2.Code != http.StatusCreated {
		t.Fatalf("повтор комментария: %d %s", c2.Code, c2.Body.String())
	}
	if jsonStr(t, c1, "id") != jsonStr(t, c2, "id") {
		t.Fatal("повтор создал второй комментарий")
	}

	// Без client_id поведение прежнее: две записи — это две записи.
	plain := map[string]any{"body": "без ключа", "entry_date": "2026-08-30"}
	a := doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+circleID+"/posts", token, plain)
	b := doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+circleID+"/posts", token, plain)
	if jsonStr(t, a, "id") == jsonStr(t, b, "id") {
		t.Fatal("записи без client_id склеились")
	}
}

// Инвариант (QLT-3): PATCH круга применяется целиком или не применяется.
// Раньше это были пять отдельных транзакций подряд, и неверное значение в
// конце оставляло круг наполовину изменённым.
func TestPatchCircleIsAllOrNothing(t *testing.T) {
	srv, caps, _, _ := setupAPI(t)
	token, _ := registerSession(t, srv, caps, "patch@example.com")
	rec := doJSON(t, srv, http.MethodPost, "/api/v1/circles", token, map[string]any{
		"name": "Семья", "owner_name": "Аня", "color": "olive",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create circle: %d %s", rec.Code, rec.Body.String())
	}
	circleID := jsonStr(t, rec, "id")

	// Имя верное, цвет — нет: не должно пройти ничего.
	rec = doJSON(t, srv, http.MethodPatch, "/api/v1/circles/"+circleID, token, map[string]any{
		"name": "Новое имя", "color": "не-цвет",
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("патч с неверным цветом: %d %s", rec.Code, rec.Body.String())
	}
	rec = doGET(t, srv, "/api/v1/circles", token)
	var list map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&list); err != nil {
		t.Fatal(err)
	}
	circles, _ := list["circles"].([]any)
	if len(circles) != 1 {
		t.Fatalf("circles: %v", list["circles"])
	}
	row, _ := circles[0].(map[string]any)
	if row["name"] != "Семья" {
		t.Fatalf("имя применилось при отказе: %v", row["name"])
	}

	// Корректный патч из нескольких полей проходит целиком.
	rec = doJSON(t, srv, http.MethodPatch, "/api/v1/circles/"+circleID, token, map[string]any{
		"name": "Новое имя", "color": "plum", "invite_who": "owner",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("корректный патч: %d %s", rec.Code, rec.Body.String())
	}
	rec = doGET(t, srv, "/api/v1/circles", token)
	if err := json.NewDecoder(rec.Body).Decode(&list); err != nil {
		t.Fatal(err)
	}
	circles, _ = list["circles"].([]any)
	row, _ = circles[0].(map[string]any)
	if row["name"] != "Новое имя" || row["color"] != "plum" {
		t.Fatalf("патч применился не целиком: %v", row)
	}
}
