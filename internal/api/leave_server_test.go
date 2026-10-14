package api_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"gitea.mixdep.ru/mix/wynd/internal/api"
	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
)

// bobSaysThings: Боб пишет запись, комментарий и реакцию, получает от
// владельца право на настройки — его имя попадает во все копии, где оно
// хранится (подписи, строки хроники, поиск).
func bobSaysThings(t *testing.T, srv *api.Server, ownerTok, memberTok, circleID, memberID string) {
	t.Helper()
	rec := doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+circleID+"/posts", memberTok, map[string]any{
		"body": "запись с дачи", "entry_date": "2026-08-30",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("запись: %d %s", rec.Code, rec.Body.String())
	}
	postID := jsonStr(t, rec, "id")
	rec = doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+circleID+"/posts/"+postID+"/comments", memberTok, map[string]any{
		"body": "и ещё",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("комментарий: %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, srv, http.MethodPut, "/api/v1/circles/"+circleID+"/posts/"+postID+"/reactions", memberTok, map[string]any{
		"emoji": "heart",
	})
	if rec.Code != http.StatusOK && rec.Code != http.StatusCreated {
		t.Fatalf("реакция: %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, srv, http.MethodPut, "/api/v1/circles/"+circleID+"/members/"+memberID, ownerTok, map[string]any{
		"can_settings": true,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("право на настройки: %d %s", rec.Code, rec.Body.String())
	}
}

// nameCopies — где в базе ещё лежит имя: строки имён, подписи, хроника, поиск.
func nameCopies(t *testing.T, ch *chronicle.Chronicle, name string) []string {
	t.Helper()
	var found []string
	for _, q := range []string{
		`SELECT 'identity_names' FROM identity_names WHERE name LIKE ?`,
		`SELECT 'posts' FROM posts WHERE author_name LIKE ?`,
		`SELECT 'comments' FROM comments WHERE author_name LIKE ?`,
		`SELECT 'reactions' FROM reactions WHERE author_name LIKE ?`,
		`SELECT 'events.actor_name' FROM events WHERE actor_name LIKE ?`,
		`SELECT 'events.summary: ' || summary FROM events WHERE summary LIKE ?`,
		`SELECT 'events.payload' FROM events WHERE payload LIKE ?`,
		`SELECT 'content_fts' FROM content_fts WHERE author_name LIKE ?`,
	} {
		rows, err := ch.DB().QueryContext(t.Context(), q, "%"+name+"%")
		if err != nil {
			t.Fatalf("%s: %v", q, err)
		}
		for rows.Next() {
			var where string
			if err := rows.Scan(&where); err != nil {
				t.Fatal(err)
			}
			found = append(found, where)
		}
		rows.Close()
	}
	return found
}

func setNameErasure(t *testing.T, srv *api.Server, allow bool) {
	t.Helper()
	rec := doJSON(t, srv, http.MethodPut, "/api/v1/admin/access", adminToken(t, srv), map[string]any{
		"allow_name_erasure": allow,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("переключатель: %d %s", rec.Code, rec.Body.String())
	}
}

// Уход насовсем (7.29, 7.31): учётки нет, вход закрыт на всех устройствах,
// а сказанное в круге осталось под прежним именем.
func TestLeaveServerKeepsWhatWasSaid(t *testing.T) {
	srv, caps, ch, _ := setupAPI(t)
	ownerTok, memberTok, circleID, memberID := circleWithMember(t, srv, caps)
	bobSaysThings(t, srv, ownerTok, memberTok, circleID, memberID)

	rec := doJSON(t, srv, http.MethodPost, "/api/v1/auth/leave", memberTok, map[string]any{})
	if rec.Code != http.StatusOK {
		t.Fatalf("уход: %d %s", rec.Code, rec.Body.String())
	}
	if got := circlesStatus(t, srv, memberTok); got != http.StatusForbidden {
		t.Fatalf("после ухода вход должен быть закрыт: %d", got)
	}
	post := firstPost(t, srv, ownerTok, circleID)
	if post["author_name"] != "Боб" {
		t.Fatalf("подпись записи: %v", post["author_name"])
	}
	for _, m := range members(t, srv, ownerTok, circleID) {
		if m["name"] == "Боб" {
			t.Fatal("ушедший с сервера не должен быть в списке участников")
		}
	}
	// Вернуться с той же почтой можно — это новый человек, без кругов.
	_, again := registerSession(t, srv, caps, "bob@example.com")
	if again == memberID {
		t.Fatal("вернувшийся получил прежнюю учётку")
	}
	if len(nameCopies(t, ch, "Боб")) == 0 {
		t.Fatal("без стирания имя должно остаться")
	}
}

// Стирание имён (7.32, 9.13): без переключателя администратора — отказ; с ним
// имя, фото и пол уходят из всех копий, записи остаются без подписи.
func TestLeaveServerErasesNamesWhenAllowed(t *testing.T) {
	srv, caps, ch, _ := setupAPI(t)
	ownerTok, memberTok, circleID, memberID := circleWithMember(t, srv, caps)
	bobSaysThings(t, srv, ownerTok, memberTok, circleID, memberID)

	rec := doJSON(t, srv, http.MethodPost, "/api/v1/auth/leave", memberTok, map[string]any{"erase_names": true})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("стирание без переключателя: %d %s", rec.Code, rec.Body.String())
	}
	if got := circlesStatus(t, srv, memberTok); got != http.StatusOK {
		t.Fatalf("отказ не должен удалять учётку: %d", got)
	}

	setNameErasure(t, srv, true)
	inst := doGET(t, srv, "/api/v1/instance", "")
	var info map[string]any
	if err := json.Unmarshal(inst.Body.Bytes(), &info); err != nil {
		t.Fatal(err)
	}
	if info["allow_name_erasure"] != true {
		t.Fatalf("instance: %v", info["allow_name_erasure"])
	}

	rec = doJSON(t, srv, http.MethodPost, "/api/v1/auth/leave", memberTok, map[string]any{"erase_names": true})
	if rec.Code != http.StatusOK {
		t.Fatalf("уход со стиранием: %d %s", rec.Code, rec.Body.String())
	}
	if left := nameCopies(t, ch, "Боб"); len(left) > 0 {
		t.Fatalf("имя осталось: %s", strings.Join(left, "; "))
	}
	post := firstPost(t, srv, ownerTok, circleID)
	if post["author_name"] != chronicle.ErasedName {
		t.Fatalf("подпись записи: %v", post["author_name"])
	}
	if post["body"] != "запись с дачи" {
		t.Fatalf("сказанное должно остаться: %v", post["body"])
	}
	// Хроника читается: строки на месте, у владельца имя осталось.
	events := syncEvents(t, srv, ownerTok)
	for _, want := range []string{
		"Аня: право менять настройки — " + chronicle.ErasedName,
		"Больше не в круге: " + chronicle.ErasedName,
		"Новая запись: " + chronicle.ErasedName,
	} {
		if !eventsContain(events, want) {
			t.Fatalf("в хронике нет строки %q", want)
		}
	}
}

// Владелец не уходит, пока у него есть круги (7.30): ничего не меняется.
func TestLeaveServerRefusesCircleOwner(t *testing.T) {
	srv, caps, _, _ := setupAPI(t)
	ownerTok, _, circleID, _ := circleWithMember(t, srv, caps)

	rec := doGET(t, srv, "/api/v1/auth/leave", ownerTok)
	var preview struct {
		Circles int `json:"circles"`
		Owned   []struct {
			Name    string `json:"name"`
			Members int    `json:"members"`
		} `json:"owned"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &preview); err != nil {
		t.Fatal(err)
	}
	if preview.Circles != 1 || len(preview.Owned) != 1 || preview.Owned[0].Name != "Семья" || preview.Owned[0].Members != 2 {
		t.Fatalf("preview: %s", rec.Body.String())
	}

	rec = doJSON(t, srv, http.MethodPost, "/api/v1/auth/leave", ownerTok, map[string]any{})
	if rec.Code != http.StatusConflict {
		t.Fatalf("уход владельца: %d %s", rec.Code, rec.Body.String())
	}
	if got := circlesStatus(t, srv, ownerTok); got != http.StatusOK {
		t.Fatalf("владелец должен остаться вошедшим: %d", got)
	}
	if len(members(t, srv, ownerTok, circleID)) != 2 {
		t.Fatal("состав круга изменился")
	}
}
