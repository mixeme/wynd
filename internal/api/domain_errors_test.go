package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
