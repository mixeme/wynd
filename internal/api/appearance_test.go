package api_test

import (
	"net/http"
	"testing"
)

// Форма аватаров (A16): по умолчанию круг, меняется, чужое значение — отказ.
func TestAppearanceAvatarShape(t *testing.T) {
	srv, caps, _, _ := setupAPI(t)
	anya, _ := registerSession(t, srv, caps, "anya@example.com")
	if got := jsonStr(t, doGET(t, srv, "/api/v1/appearance", anya), "avatar_shape"); got != "circle" {
		t.Fatalf("по умолчанию: %q", got)
	}
	if rec := doJSON(t, srv, http.MethodPut, "/api/v1/appearance", anya, map[string]string{"avatar_shape": "square"}); rec.Code != http.StatusOK {
		t.Fatalf("сменить: %d %s", rec.Code, rec.Body.String())
	}
	if got := jsonStr(t, doGET(t, srv, "/api/v1/appearance", anya), "avatar_shape"); got != "square" {
		t.Fatalf("после смены: %q", got)
	}
	if rec := doJSON(t, srv, http.MethodPut, "/api/v1/appearance", anya, map[string]string{"avatar_shape": "hexagon"}); rec.Code != http.StatusBadRequest {
		t.Fatalf("чужая форма: %d %s", rec.Code, rec.Body.String())
	}
}
