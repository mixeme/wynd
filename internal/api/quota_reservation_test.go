package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
)

// R3: проверка места и запись — одно действие. Двадцать четыре
// одновременных записи при месте ровно на одну создают одну, а не столько,
// сколько успели прочитать одинаковое свободное место.
func TestConcurrentTextWritesRespectQuota(t *testing.T) {
	srv, caps, _, blobs := setupAPI(t)
	_, memberTok, circleID, _ := circleWithMember(t, srv, caps)
	base := "/api/v1/circles/" + circleID
	before := circleUsed(t, blobs, circleID)
	setCircleQuota(t, blobs, circleID, before+chronicle.PostTextCost("x", "Боб", "2026-08-30", nil, chronicle.GenderNone))

	payload, err := json.Marshal(map[string]any{"body": "x", "entry_date": "2026-08-30"})
	if err != nil {
		t.Fatal(err)
	}
	const writers = 24
	codes := make([]int, writers)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodPost, base+"/posts", bytes.NewReader(payload))
			req.Header.Set("Authorization", "Bearer "+memberTok)
			rec := httptest.NewRecorder()
			<-start
			srv.ServeHTTP(rec, req)
			codes[i] = rec.Code
		}(i)
	}
	close(start)
	wg.Wait()

	created := 0
	for _, code := range codes {
		switch code {
		case http.StatusCreated:
			created++
		case http.StatusInsufficientStorage:
		default:
			t.Fatalf("неожиданный код ответа: %d", code)
		}
	}
	if created != 1 {
		t.Fatalf("создано %d записей при месте на одну", created)
	}
	if got := len(feedPosts(t, srv, memberTok, circleID)); got != 1 {
		t.Fatalf("в ленте %d записей, ждали одну", got)
	}
}

// R6: оценка записи включает имя автора и строку события. При месте,
// посчитанном по прежней заниженной формуле, запись отклоняется, а при
// точной — укладывается ровно, без переполнения.
func TestExactTextCostFitsQuota(t *testing.T) {
	srv, caps, _, blobs := setupAPI(t)
	_, memberTok, circleID, _ := circleWithMember(t, srv, caps)
	base := "/api/v1/circles/" + circleID
	before := circleUsed(t, blobs, circleID)

	// Прежняя формула: 550 + 400 + 4*len("x") = 954. Этого не хватает.
	setCircleQuota(t, blobs, circleID, before+954)
	if rec := doJSON(t, srv, http.MethodPost, base+"/posts", memberTok, map[string]any{
		"body": "x", "entry_date": "2026-08-30",
	}); rec.Code != http.StatusInsufficientStorage {
		t.Fatalf("запись по заниженной оценке: %d %s", rec.Code, rec.Body.String())
	}

	cost := chronicle.PostTextCost("x", "Боб", "2026-08-30", nil, chronicle.GenderNone)
	setCircleQuota(t, blobs, circleID, before+cost)
	if rec := doJSON(t, srv, http.MethodPost, base+"/posts", memberTok, map[string]any{
		"body": "x", "entry_date": "2026-08-30",
	}); rec.Code != http.StatusCreated {
		t.Fatalf("запись по точной оценке: %d %s", rec.Code, rec.Body.String())
	}
	if used := circleUsed(t, blobs, circleID); used != before+cost {
		t.Fatalf("занято %d, ждали ровно %d", used, before+cost)
	}
}
