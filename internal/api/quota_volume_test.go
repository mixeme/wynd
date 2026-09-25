package api_test

import (
	"net/http"
	"strings"
	"testing"
)

// Инвариант: у круга без медиа объём по месяцам — пустой массив, а не null.
// null ронял экран «Архив и очистка»: клиент читал volume.length (найдено при
// проверке плана 42 в сборке).
func TestCircleQuotaVolumeIsEmptyArray(t *testing.T) {
	srv, caps, _, _ := setupAPI(t)
	tok, _ := registerSession(t, srv, caps, "volume@example.com")
	rec := doJSON(t, srv, http.MethodPost, "/api/v1/circles", tok, map[string]any{
		"name": "Семья", "owner_name": "Аня",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("circle: %d %s", rec.Code, rec.Body.String())
	}
	circleID := jsonStr(t, rec, "id")
	q := doGET(t, srv, "/api/v1/circles/"+circleID+"/quota", tok)
	if q.Code != http.StatusOK {
		t.Fatalf("quota: %d %s", q.Code, q.Body.String())
	}
	if !strings.Contains(q.Body.String(), `"volume":[]`) {
		t.Fatalf("volume не пустой массив: %s", q.Body.String())
	}
}
