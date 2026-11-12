package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
	"gitea.mixdep.ru/mix/wynd/internal/mail"
	"gitea.mixdep.ru/mix/wynd/internal/push"
)

func TestWriteDomainErrorNotConfigured(t *testing.T) {
	cases := []struct {
		err  error
		code string
	}{
		{mail.ErrNotConfigured, "smtp_not_configured"},
		{mail.ErrInvalid, "invalid"},
		{push.ErrNotConfigured, "push_not_configured"},
	}
	for _, tc := range cases {
		rec := httptest.NewRecorder()
		writeDomainError(rec, tc.err)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%v: status %d", tc.err, rec.Code)
		}
		var body map[string]string
		if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["error"] != tc.code {
			t.Fatalf("%v: error=%q want %q", tc.err, body["error"], tc.code)
		}
	}
}

// C81: сервер знает, что данные круга не читаются, — уходит отдельный код,
// а не «invalid» с подсказкой про адрес. Наружу — только код, подробности в
// журнал.
func TestWriteDomainErrorArchiveTables(t *testing.T) {
	rec := httptest.NewRecorder()
	writeDomainError(rec, fmt.Errorf("%w: post_media.crop_x unsupported column", chronicle.ErrArchiveTables))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d", rec.Code)
	}
	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body["error"] != "archive_invalid" {
		t.Fatalf("error=%q want archive_invalid", body["error"])
	}
	if strings.Contains(rec.Body.String(), "crop_x") {
		t.Fatalf("подробности ушли наружу: %s", rec.Body.String())
	}
}

// Инвариант (аудит 2026-09-22): наружу уходит классифицированная причина,
// а не сырая строка сервера почты — она попадала в панель как есть.
func TestWriteDomainErrorSMTPSend(t *testing.T) {
	rec := httptest.NewRecorder()
	writeDomainError(rec, &mail.SendError{
		Reason: mail.ReasonDial,
		Err:    fmt.Errorf("dial smtp.internal.lan:587: i/o timeout"),
	})
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status %d", rec.Code)
	}
	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body["error"] != "smtp_failed" {
		t.Fatalf("error=%q", body["error"])
	}
	if body["detail"] != mail.ReasonDial {
		t.Fatalf("detail=%q want %q", body["detail"], mail.ReasonDial)
	}
	if strings.Contains(rec.Body.String(), "smtp.internal.lan") {
		t.Fatalf("сырой ответ сервера ушёл наружу: %s", rec.Body.String())
	}
}
