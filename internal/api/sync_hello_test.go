package api_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// Инвариант (план 42, BKP-3): курсор больше последнего события — база
// восстановлена из бэкапа. Поток начинается кадром hello с max_seq, а курсор
// сервер считает равным max_seq: новые события доходят сразу, а не после
// того, как счётчик догонит старый номер.
func TestSyncHelloResetsStaleCursor(t *testing.T) {
	srv, caps, ch, _ := setupAPI(t)
	token, _ := registerSession(t, srv, caps, "hello@example.com")
	rec0 := doJSON(t, srv, http.MethodPost, "/api/v1/circles", token, map[string]any{
		"name": "Семья", "owner_name": "Аня",
	})
	if rec0.Code != http.StatusCreated {
		t.Fatalf("create circle: %d %s", rec0.Code, rec0.Body.String())
	}
	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rec0.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	maxSeq, err := ch.MaxEventSeq(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	// SSE: hello первым кадром, затем событие, созданное уже после подключения.
	ctx, cancel := context.WithTimeout(t.Context(), 2600*time.Millisecond)
	defer cancel()
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/sync?cursor=999999", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "text/event-stream")
	rec := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		defer close(done)
		srv.ServeHTTP(rec, req)
	}()
	time.Sleep(100 * time.Millisecond)
	recPost := doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+created.ID+"/posts", token, map[string]any{
		"body": "после восстановления", "entry_date": "2026-09-24",
	})
	if recPost.Code != http.StatusCreated {
		t.Fatalf("create post: %d %s", recPost.Code, recPost.Body.String())
	}
	<-done

	body := rec.Body.String()
	hello := fmt.Sprintf("event: hello\ndata: {\"max_seq\":%d}\n\n", maxSeq)
	if !strings.HasPrefix(body, hello) {
		t.Fatalf("поток не начинается с hello:\n%s", body)
	}
	if !strings.Contains(body, "после восстановления") {
		t.Fatalf("событие после сброса курсора не дошло:\n%s", body)
	}
}
