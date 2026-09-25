package api_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"gitea.mixdep.ru/mix/wynd/internal/api"
)

// Инвариант: пароль релея не возвращается наружу ни одним ответом панели, и
// пустое поле при сохранении означает «оставить прежний», а не «стереть».
func TestAdminSetSMTPNeverEchoesPassword(t *testing.T) {
	srv, _, _, _ := setupAPI(t)
	admin := adminToken(t, srv)
	const secret = "очень-секретный-пароль-релея"

	rec := doJSON(t, srv, http.MethodPut, "/api/v1/admin/smtp", admin, map[string]any{
		// Loopback и закрытый порт: проверка релея падает сразу, а не по таймауту DNS.
		"host": "127.0.0.1", "port": 1,
		"username": "wynd@example.org", "password": secret, "from": "Wynd <wynd@example.org>",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("сохранение релея: %d %s", rec.Code, rec.Body.String())
	}

	// Ни чтение настроек, ни диагностика пароль не показывают.
	if rec := doGET(t, srv, "/api/v1/admin/smtp", admin); rec.Code != http.StatusOK {
		t.Fatalf("чтение настроек: %d %s", rec.Code, rec.Body.String())
	} else if strings.Contains(rec.Body.String(), secret) {
		t.Fatalf("настройки вернули пароль: %s", rec.Body.String())
	}
	if rec := doJSON(t, srv, http.MethodPost, "/api/v1/admin/check", admin, map[string]any{}); strings.Contains(rec.Body.String(), secret) {
		t.Fatalf("диагностика вернула пароль: %s", rec.Body.String())
	}
	rec = doJSON(t, srv, http.MethodPost, "/api/v1/admin/smtp/test", admin, map[string]any{
		"to": "ana@example.org",
	})
	if strings.Contains(rec.Body.String(), secret) {
		t.Fatalf("ответ проверки релея вернул пароль: %s", rec.Body.String())
	}

	// Пустой пароль в запросе — сохранить прежний.
	rec = doJSON(t, srv, http.MethodPut, "/api/v1/admin/smtp", admin, map[string]any{
		"host": "127.0.0.1", "port": 2525,
		"username": "wynd@example.org", "password": "", "from": "Wynd <wynd@example.org>",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("повторное сохранение: %d %s", rec.Code, rec.Body.String())
	}
	cfg, err := srv.Mail.LoadConfig(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Password != secret {
		t.Fatalf("пароль потерян при сохранении без него: %q", cfg.Password)
	}
	if cfg.Port != 2525 {
		t.Fatalf("остальные поля не применились: %+v", cfg)
	}
}

// Инвариант: заявка на квоту решается один раз, и решение видно в квоте
// круга; отозванная серверная ссылка перестаёт работать.
func TestAdminQuotaRequestApproveRejectAndInviteRevoke(t *testing.T) {
	srv, caps, _, _ := setupAPI(t)
	admin := adminToken(t, srv)
	ownerTok, _ := registerSession(t, srv, caps, "anya@example.com")
	rec := doJSON(t, srv, http.MethodPost, "/api/v1/circles", ownerTok, map[string]any{
		"name": "Семья", "owner_name": "Аня",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("круг: %d %s", rec.Code, rec.Body.String())
	}
	circleID := jsonStr(t, rec, "id")

	const gb = int64(1) << 30
	request := func(bytes int64) string {
		t.Helper()
		rec := doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+circleID+"/quota_requests", ownerTok, map[string]any{
			"requested_bytes": bytes,
		})
		if rec.Code != http.StatusCreated {
			t.Fatalf("заявка на квоту: %d %s", rec.Code, rec.Body.String())
		}
		return jsonStr(t, rec, "id")
	}
	quotaBytes := func() float64 {
		t.Helper()
		rec := doGET(t, srv, "/api/v1/circles/"+circleID+"/quota", ownerTok)
		if rec.Code != http.StatusOK {
			t.Fatalf("квота круга: %d %s", rec.Code, rec.Body.String())
		}
		var out map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
		v, _ := out["quota_bytes"].(float64)
		return v
	}
	pending := func() []any {
		t.Helper()
		rec := doGET(t, srv, "/api/v1/admin/quota_requests", admin)
		if rec.Code != http.StatusOK {
			t.Fatalf("заявки в панели: %d %s", rec.Code, rec.Body.String())
		}
		var out map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
		items, _ := out["requests"].([]any)
		return items
	}

	// Отказ: квота не меняется, заявка уходит из ожидающих и второй раз не решается.
	before := quotaBytes()
	rejected := request(7 * gb)
	if len(pending()) != 1 {
		t.Fatalf("заявка не видна в панели: %+v", pending())
	}
	rec = doJSON(t, srv, http.MethodPost, "/api/v1/admin/quota_requests/"+rejected+"/reject", admin, map[string]any{
		"admin_note": "места нет",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("отказ: %d %s", rec.Code, rec.Body.String())
	}
	if got := quotaBytes(); got != before {
		t.Fatalf("отказ изменил квоту: %v → %v", before, got)
	}
	if len(pending()) != 0 {
		t.Fatalf("решённая заявка осталась ожидающей: %+v", pending())
	}
	if rec := doJSON(t, srv, http.MethodPost, "/api/v1/admin/quota_requests/"+rejected+"/approve", admin, nil); rec.Code != http.StatusNotFound {
		t.Fatalf("повторное решение по заявке: %d %s", rec.Code, rec.Body.String())
	}

	// Утверждение: квота круга становится запрошенной.
	approved := request(9 * gb)
	rec = doJSON(t, srv, http.MethodPost, "/api/v1/admin/quota_requests/"+approved+"/approve", admin, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("утверждение: %d %s", rec.Code, rec.Body.String())
	}
	if got := quotaBytes(); int64(got) != 9*gb {
		t.Fatalf("квота после утверждения: %v", got)
	}
	if len(pending()) != 0 {
		t.Fatalf("утверждённая заявка осталась ожидающей: %+v", pending())
	}

	// Серверная ссылка: до отзыва работает, после — нет.
	rec = doJSON(t, srv, http.MethodPost, "/api/v1/admin/invites", admin, map[string]any{
		"kind": "multi", "max_uses": 5, "ttl_sec": 3600,
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("серверная ссылка: %d %s", rec.Code, rec.Body.String())
	}
	inviteToken := jsonStr(t, rec, "token")
	inviteID := serverInviteID(t, srv, admin, inviteToken)
	if rec := doGET(t, srv, "/api/v1/invites/"+inviteToken, ""); rec.Code != http.StatusOK {
		t.Fatalf("просмотр живой ссылки: %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, srv, http.MethodDelete, "/api/v1/admin/invites/"+inviteID, admin, nil); rec.Code != http.StatusOK {
		t.Fatalf("отзыв ссылки: %d %s", rec.Code, rec.Body.String())
	}
	if rec := doGET(t, srv, "/api/v1/invites/"+inviteToken, ""); rec.Code == http.StatusOK {
		t.Fatalf("отозванная ссылка всё ещё открывается: %s", rec.Body.String())
	}
	rec = doJSON(t, srv, http.MethodPost, "/api/v1/invites/"+inviteToken+"/accept", "", map[string]string{
		"email": "zoe@example.com",
	})
	if rec.Code == http.StatusAccepted {
		t.Fatalf("по отозванной ссылке выдан код: %s", rec.Body.String())
	}
	if rec := doJSON(t, srv, http.MethodDelete, "/api/v1/admin/invites/"+inviteID, admin, nil); rec.Code != http.StatusNotFound {
		t.Fatalf("повторный отзыв: %d %s", rec.Code, rec.Body.String())
	}
}

// serverInviteID находит id серверной ссылки по токену: ответ на создание
// отдаёт только токен и срок.
func serverInviteID(t *testing.T, srv *api.Server, admin, token string) string {
	t.Helper()
	rec := doGET(t, srv, "/api/v1/admin/invites", admin)
	if rec.Code != http.StatusOK {
		t.Fatalf("список ссылок: %d %s", rec.Code, rec.Body.String())
	}
	var out struct {
		Invites []map[string]any `json:"invites"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	for _, inv := range out.Invites {
		if inv["token"] == token {
			return inv["id"].(string)
		}
	}
	t.Fatalf("ссылка не найдена в списке: %s", rec.Body.String())
	return ""
}
