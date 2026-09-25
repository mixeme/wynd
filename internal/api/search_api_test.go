package api_test

import (
	"encoding/json"
	"net/http"
	"net/url"
	"testing"

	"gitea.mixdep.ru/mix/wynd/internal/api"
)

func searchHits(t *testing.T, srv *api.Server, token, path string) []map[string]any {
	t.Helper()
	rec := doGET(t, srv, path, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("поиск %s: %d %s", path, rec.Code, rec.Body.String())
	}
	var out struct {
		Hits []map[string]any `json:"hits"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out.Hits
}

func hitBodies(hits []map[string]any) []string {
	out := make([]string, 0, len(hits))
	for _, h := range hits {
		if s, ok := h["snippet"].(string); ok {
			out = append(out, s)
			continue
		}
		if s, ok := h["body"].(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func hasHit(hits []map[string]any, postID string) bool {
	for _, h := range hits {
		if h["post_id"] == postID {
			return true
		}
	}
	return false
}

// Инвариант: обработчики поиска разбирают фильтры из запроса и не выносят
// круг за пределы членства. Три обработчика (круг, авторы, общий поиск) до
// этого не исполнялись тестами ни разу.
func TestSearchRespectsMembershipAndFilters(t *testing.T) {
	srv, caps, _, _ := setupAPI(t)
	ownerTok, memberTok, circleID, _ := circleWithMember(t, srv, caps)

	post := func(token, body, date string, media []map[string]any) string {
		t.Helper()
		payload := map[string]any{"body": body, "entry_date": date}
		if media != nil {
			payload["media"] = media
		}
		rec := doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+circleID+"/posts", token, payload)
		if rec.Code != http.StatusCreated {
			t.Fatalf("запись %q: %d %s", body, rec.Code, rec.Body.String())
		}
		return jsonStr(t, rec, "id")
	}

	plainID := post(ownerTok, "альфа без фото", "2026-01-15", nil)

	photoBlob := uploadBlob(t, srv, ownerTok, []byte("photo-bytes"), "image/jpeg")
	photoID := post(ownerTok, "альфа со снимком", "2026-03-20", []map[string]any{
		{"blob_id": photoBlob, "kind": "photo"},
	})

	geoBlob := uploadBlob(t, srv, memberTok, []byte("geo-bytes-1"), "image/jpeg")
	geoID := post(memberTok, "альфа с местом", "2026-06-01", []map[string]any{
		{"blob_id": geoBlob, "kind": "photo", "geo_lat": 55.75, "geo_lng": 37.62},
	})

	search := "/api/v1/circles/" + circleID + "/search?q=" + url.QueryEscape("альфа")

	all := searchHits(t, srv, ownerTok, search)
	if len(all) != 3 {
		t.Fatalf("без фильтров найдено %d: %v", len(all), hitBodies(all))
	}

	// Отрезок дат.
	ranged := searchHits(t, srv, ownerTok, search+"&from=2026-03-01&to=2026-04-01")
	if len(ranged) != 1 || !hasHit(ranged, photoID) {
		t.Fatalf("фильтр по датам: %v", hitBodies(ranged))
	}

	// Только с фото.
	photos := searchHits(t, srv, ownerTok, search+"&has_photo=1")
	if len(photos) != 2 || hasHit(photos, plainID) {
		t.Fatalf("фильтр has_photo: %v", hitBodies(photos))
	}

	// Только с местом.
	located := searchHits(t, srv, ownerTok, search+"&has_location=1")
	if len(located) != 1 || !hasHit(located, geoID) {
		t.Fatalf("фильтр has_location: %v", hitBodies(located))
	}

	// По автору.
	byAuthor := searchHits(t, srv, ownerTok, search+"&author="+url.QueryEscape("Боб"))
	if len(byAuthor) != 1 || !hasHit(byAuthor, geoID) {
		t.Fatalf("фильтр author: %v", hitBodies(byAuthor))
	}

	// Список авторов — из попаданий, а не из состава круга.
	rec := doGET(t, srv, "/api/v1/circles/"+circleID+"/search/authors?q="+url.QueryEscape("альфа"), ownerTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("авторы: %d %s", rec.Code, rec.Body.String())
	}
	var authors struct {
		Authors []string `json:"authors"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &authors); err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, a := range authors.Authors {
		seen[a] = true
	}
	if len(authors.Authors) != 2 || !seen["Аня"] || !seen["Боб"] {
		t.Fatalf("авторы попаданий: %v", authors.Authors)
	}

	// Общий поиск участника видит свой круг…
	mine := searchHits(t, srv, memberTok, "/api/v1/search?q="+url.QueryEscape("альфа"))
	if len(mine) != 3 {
		t.Fatalf("общий поиск участника: %v", hitBodies(mine))
	}
	// …а посторонний — ничего, ни в круге, ни в общем поиске.
	strangerTok, _ := registerSession(t, srv, caps, "zoe@example.com")
	if hits := searchHits(t, srv, strangerTok, "/api/v1/search?q="+url.QueryEscape("альфа")); len(hits) != 0 {
		t.Fatalf("общий поиск постороннего: %v", hitBodies(hits))
	}
	// Поиск по чужому кругу отвечает «не найдено» — существование круга
	// постороннему не подтверждается.
	if rec := doGET(t, srv, search, strangerTok); rec.Code != http.StatusNotFound {
		t.Fatalf("поиск постороннего по кругу: %d %s", rec.Code, rec.Body.String())
	}
}
