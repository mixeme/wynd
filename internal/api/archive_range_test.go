package api_test

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

// rangeArchive — GET архива участника с дополнительными заголовками.
func rangeArchive(t *testing.T, srv interface {
	ServeHTTP(w http.ResponseWriter, r *http.Request)
}, tok, circleID string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/circles/"+circleID+"/archive/download", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	return rec
}

// Докачка (план 66, функция 5): ETag и Accept-Ranges у полного ответа;
// за пределами файла — 416; If-Range с чужим валидатором — начало заново.
func TestArchiveRangeEdges(t *testing.T) {
	srv, caps, _, _ := setupAPI(t)
	tok, _ := registerSession(t, srv, caps, "range-edge@example.com")
	rec := doJSON(t, srv, http.MethodPost, "/api/v1/circles", tok, map[string]any{
		"name": "Семья", "owner_name": "Аня",
	})
	circleID := jsonStr(t, rec, "id")
	rec = doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+circleID+"/posts", tok, map[string]any{
		"body": "запись для докачки", "entry_date": "2026-08-02",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("post: %d %s", rec.Code, rec.Body.String())
	}
	startArchive(t, srv, tok, circleID)

	full := rangeArchive(t, srv, tok, circleID, nil)
	if full.Code != http.StatusOK {
		t.Fatalf("полный: %d %s", full.Code, full.Body.String())
	}
	etag := full.Header().Get("ETag")
	if etag == "" || !strings.HasPrefix(etag, `"`) {
		t.Fatalf("ETag = %q", etag)
	}
	if got := full.Header().Get("Accept-Ranges"); got != "bytes" {
		t.Fatalf("Accept-Ranges = %q", got)
	}
	size := full.Body.Len()

	// За пределами файла — 416 с длиной.
	out := rangeArchive(t, srv, tok, circleID, map[string]string{"Range": "bytes=" + strconv.Itoa(size+10) + "-"})
	if out.Code != http.StatusRequestedRangeNotSatisfiable {
		t.Fatalf("за пределами: %d, ждали 416", out.Code)
	}
	if got := out.Header().Get("Content-Range"); got != "bytes */"+strconv.Itoa(size) {
		t.Fatalf("416 Content-Range = %q", got)
	}

	// Чужой If-Range — сервер обязан отдать весь файл, а не продолжение.
	mismatch := rangeArchive(t, srv, tok, circleID, map[string]string{
		"Range": "bytes=10-", "If-Range": `"чужой-валидатор"`,
	})
	if mismatch.Code != http.StatusOK || mismatch.Body.Len() != size {
		t.Fatalf("чужой If-Range: %d, %d байт", mismatch.Code, mismatch.Body.Len())
	}

	// Свой If-Range — продолжение.
	match := rangeArchive(t, srv, tok, circleID, map[string]string{
		"Range": "bytes=10-", "If-Range": etag,
	})
	if match.Code != http.StatusPartialContent || match.Body.Len() != size-10 {
		t.Fatalf("свой If-Range: %d, %d байт", match.Code, match.Body.Len())
	}

	// Range=0 — весь файл обычным ответом.
	zero := rangeArchive(t, srv, tok, circleID, map[string]string{"Range": "bytes=0-"})
	if zero.Code != http.StatusOK || zero.Body.Len() != size {
		t.Fatalf("bytes=0-: %d, %d байт", zero.Code, zero.Body.Len())
	}
}

// Ссылка с докачкой: хвост с Range — это конец файла, и ссылка после него
// отработана: второй раз по ней ничего не придёт.
func TestArchiveLinkResumeFinishesByRange(t *testing.T) {
	srv, caps, _, _ := setupAPI(t)
	tok, _ := registerSession(t, srv, caps, "link-resume@example.com")
	rec := doJSON(t, srv, http.MethodPost, "/api/v1/circles", tok, map[string]any{
		"name": "Дача", "owner_name": "Аня",
	})
	circleID := jsonStr(t, rec, "id")
	rec = doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+circleID+"/posts", tok, map[string]any{
		"body": "запись для ссылки", "entry_date": "2026-08-03",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("post: %d %s", rec.Code, rec.Body.String())
	}
	startArchive(t, srv, tok, circleID)
	rec = doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+circleID+"/archive/link", tok, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("link: %d %s", rec.Code, rec.Body.String())
	}
	url := jsonStr(t, rec, "url")

	// Продолжение по ссылке — 206; хвост = конец файла, ссылка отработала.
	partial := doGETHeaders(t, srv, url, map[string]string{"Range": "bytes=5-"})
	if partial.Code != http.StatusPartialContent {
		t.Fatalf("продолжение по ссылке: %d %s", partial.Code, partial.Body.String())
	}
	if !strings.HasPrefix(partial.Header().Get("Content-Range"), "bytes 5-") {
		t.Fatalf("Content-Range = %q", partial.Header().Get("Content-Range"))
	}
	if again := doGET(t, srv, url, ""); again.Code != http.StatusNotFound {
		t.Fatalf("ссылка после докачанного хвоста: %d, ждали 404", again.Code)
	}
}

// doGETHeaders — GET без заголовка входа, с дополнительными заголовками.
func doGETHeaders(t *testing.T, srv interface {
	ServeHTTP(w http.ResponseWriter, r *http.Request)
}, url string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, url, nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	return rec
}
