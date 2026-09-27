package api_test

import (
	"net/http"
	"testing"
)

// Аудит 2026-09-27: карточку круга получает только читатель. Исключённый
// видел настройки приглашений, окно правки и своё лицо в круге, хотя лента,
// дни и архив ему уже закрыты.
func TestCircleDetailForbiddenForExcluded(t *testing.T) {
	srv, caps, _, _ := setupAPI(t)
	ownerTok, memberTok, circleID, memberID := circleWithMember(t, srv, caps)
	if r := doGET(t, srv, "/api/v1/circles/"+circleID, memberTok); r.Code != http.StatusOK {
		t.Fatalf("участник до исключения: %d %s", r.Code, r.Body.String())
	}
	rec := doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+circleID+"/exclude", ownerTok, map[string]any{
		"account_id": memberID,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("исключение: %d %s", rec.Code, rec.Body.String())
	}
	if r := doGET(t, srv, "/api/v1/circles/"+circleID, memberTok); r.Code != http.StatusForbidden {
		t.Fatalf("исключённый: %d %s", r.Code, r.Body.String())
	}
}
