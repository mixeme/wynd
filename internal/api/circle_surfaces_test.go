package api_test

import (
	"net/http"
	"strings"
	"testing"
)

// Инвариант: посторонний не читает круг ни с одной поверхности. Их десять,
// и каждая тянет данные своим запросом — забытая проверка видна только
// отдельным обходом всех.
func TestNonMemberCannotReadCircleSurfaces(t *testing.T) {
	srv, caps, _, _ := setupAPI(t)
	ownerTok, _, circleID, _ := circleWithMember(t, srv, caps)

	rec := doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+circleID+"/posts", ownerTok, map[string]any{
		"body": "секрет круга", "entry_date": "2026-08-30",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("запись: %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, srv, http.MethodPut, "/api/v1/circles/"+circleID+"/days/2026-08-30/title", ownerTok, map[string]any{
		"title": "секретный день",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("название дня: %d %s", rec.Code, rec.Body.String())
	}

	strangerTok, _ := registerSession(t, srv, caps, "zoe@example.com")

	surfaces := []string{
		"",
		"/feed",
		"/grid",
		"/map",
		"/days",
		"/days/2026-08-30",
		"/search?q=%D1%81%D0%B5%D0%BA%D1%80%D0%B5%D1%82",
		"/search/authors?q=%D1%81%D0%B5%D0%BA%D1%80%D0%B5%D1%82",
		"/members",
		"/identity",
	}
	for _, path := range surfaces {
		rec := doGET(t, srv, "/api/v1/circles/"+circleID+path, strangerTok)
		if rec.Code != http.StatusForbidden && rec.Code != http.StatusNotFound {
			t.Fatalf("%q: %d %s, ожидался отказ", path, rec.Code, rec.Body.String())
		}
		if strings.Contains(rec.Body.String(), "секрет") {
			t.Fatalf("%q отдала содержимое круга: %s", path, rec.Body.String())
		}
	}

	// Общий поиск по всем своим кругам тоже не достаёт до чужого.
	rec = doGET(t, srv, "/api/v1/search?q=%D1%81%D0%B5%D0%BA%D1%80%D0%B5%D1%82", strangerTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("общий поиск: %d %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "секрет") {
		t.Fatalf("общий поиск отдал чужой круг: %s", rec.Body.String())
	}

	// Круга нет и в списке кругов постороннего.
	rec = doGET(t, srv, "/api/v1/circles", strangerTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("список кругов: %d %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), circleID) {
		t.Fatalf("чужой круг в списке: %s", rec.Body.String())
	}
}
