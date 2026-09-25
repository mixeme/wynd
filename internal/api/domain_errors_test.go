package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
