package api_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"gitea.mixdep.ru/mix/wynd/internal/api"
	"gitea.mixdep.ru/mix/wynd/internal/blob"
)

func setCircleQuota(t *testing.T, blobs *blob.Store, circleID string, bytes int64) {
	t.Helper()
	if _, err := blobs.DB().ExecContext(t.Context(),
		`UPDATE circles SET quota_custom = 1, quota_bytes = ? WHERE id = ?`, bytes, circleID); err != nil {
		t.Fatal(err)
	}
}

func circleUsed(t *testing.T, blobs *blob.Store, circleID string) int64 {
	t.Helper()
	used, err := blobs.CircleUsedBytes(t.Context(), circleID)
	if err != nil {
		t.Fatal(err)
	}
	return used
}

func expectFull(t *testing.T, srv *api.Server, what, method, path, token string, body any) {
	t.Helper()
	rec := doJSON(t, srv, method, path, token, body)
	if rec.Code != http.StatusInsufficientStorage || !strings.Contains(rec.Body.String(), `"detail":"circle"`) {
		t.Fatalf("%s в упёршемся круге: %d %s", what, rec.Code, rec.Body.String())
	}
}

// Квота круга считает и текст (C42, решения владельца 2026-10-13/14): в
// упёршемся круге нельзя писать, комментировать, ставить и снимать реакции,
// править и называть день; читать и удалять своё — можно, и удаление
// освобождает место.
func TestTextQuotaFullCircle(t *testing.T) {
	srv, caps, _, blobs := setupAPI(t)
	ownerTok, memberTok, circleID, _ := circleWithMember(t, srv, caps)
	base := "/api/v1/circles/" + circleID
	before := circleUsed(t, blobs, circleID)

	rec := doJSON(t, srv, http.MethodPost, base+"/posts", memberTok, map[string]any{
		"body": strings.Repeat("слово ", 200), "entry_date": "2026-08-30", "client_id": "q-1",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("запись: %d %s", rec.Code, rec.Body.String())
	}
	postID := jsonStr(t, rec, "id")
	grown := circleUsed(t, blobs, circleID) - before
	// 2200 байт текста: строка, событие и поиск с его словарём — вчетверо
	// больше и около килобайта служебного (замер C42).
	if text := int64(len(strings.Repeat("слово ", 200))); grown < 4*text || grown > 4*text+2000 {
		t.Fatalf("запись заняла %d байт", grown)
	}
	rec = doJSON(t, srv, http.MethodPut, base+"/posts/"+postID+"/reactions", ownerTok, map[string]any{"emoji": "heart"})
	if rec.Code != http.StatusOK {
		t.Fatalf("реакция: %d %s", rec.Code, rec.Body.String())
	}

	setCircleQuota(t, blobs, circleID, circleUsed(t, blobs, circleID))

	// Улочка знает, что места нет, и кто может помочь (3.10).
	for _, who := range []struct {
		tok string
		own bool
	}{{memberTok, false}, {ownerTok, true}} {
		rec := doGET(t, srv, "/api/v1/circles", who.tok)
		body := rec.Body.String()
		if !strings.Contains(body, `"space_full":"circle"`) || !strings.Contains(body, `"owner_name":"Аня"`) ||
			!strings.Contains(body, fmt.Sprintf(`"you_own":%v`, who.own)) {
			t.Fatalf("список кругов: %s", body)
		}
	}

	expectFull(t, srv, "запись", http.MethodPost, base+"/posts", memberTok,
		map[string]any{"body": "ещё", "entry_date": "2026-08-30"})
	expectFull(t, srv, "комментарий", http.MethodPost, base+"/posts/"+postID+"/comments", ownerTok,
		map[string]any{"body": "ответ"})
	expectFull(t, srv, "правка", http.MethodPatch, base+"/posts/"+postID, memberTok,
		map[string]any{"body": "правка", "entry_date": "2026-08-30"})
	expectFull(t, srv, "реакция", http.MethodPut, base+"/posts/"+postID+"/reactions", memberTok,
		map[string]any{"emoji": "laugh"})
	expectFull(t, srv, "снять реакцию", http.MethodDelete, base+"/posts/"+postID+"/reactions", ownerTok, nil)
	expectFull(t, srv, "название дня", http.MethodPut, base+"/days/2026-08-30/title", memberTok,
		map[string]any{"title": "Дача"})

	// Повтор из очереди той же записи — не новая запись, место ей не нужно.
	rec = doJSON(t, srv, http.MethodPost, base+"/posts", memberTok, map[string]any{
		"body": strings.Repeat("слово ", 200), "entry_date": "2026-08-30", "client_id": "q-1",
	})
	if rec.Code != http.StatusCreated && rec.Code != http.StatusOK {
		t.Fatalf("повтор из очереди: %d %s", rec.Code, rec.Body.String())
	}

	// Читать можно.
	if got := len(feedPosts(t, srv, ownerTok, circleID)); got != 1 {
		t.Fatalf("лента: %d записей", got)
	}
	// Удалить своё можно, и место освобождается.
	full := circleUsed(t, blobs, circleID)
	rec = doJSON(t, srv, http.MethodDelete, base+"/posts/"+postID, memberTok, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("удаление своей записи: %d %s", rec.Code, rec.Body.String())
	}
	if after := circleUsed(t, blobs, circleID); after >= full-2*2200 {
		t.Fatalf("удаление не освободило место: было %d, стало %d", full, after)
	}
	rec = doJSON(t, srv, http.MethodPost, base+"/posts", memberTok, map[string]any{
		"body": "место появилось", "entry_date": "2026-08-30",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("запись после удаления: %d %s", rec.Code, rec.Body.String())
	}
}

// Шкала сервера и сумма кругов — одно число (C42): текст входит в занятое
// на сервере, а не только в круг.
func TestTextQuotaCountsOnServerScale(t *testing.T) {
	srv, caps, _, blobs := setupAPI(t)
	_, memberTok, circleID, _ := circleWithMember(t, srv, caps)
	before, err := blobs.UsedBytes(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	circleBefore := circleUsed(t, blobs, circleID)
	rec := doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+circleID+"/posts", memberTok, map[string]any{
		"body": "были на даче", "entry_date": "2026-08-30",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("запись: %d %s", rec.Code, rec.Body.String())
	}
	after, err := blobs.UsedBytes(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if got, want := after-before, circleUsed(t, blobs, circleID)-circleBefore; got != want || got <= 0 {
		t.Fatalf("сервер вырос на %d, круг — на %d", got, want)
	}
}
