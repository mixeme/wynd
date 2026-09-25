package api_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Инвариант: три вида одного потока — JSON, SSE и NDJSON — отдают одни и те
// же события в одном порядке. Расхождение видно только сравнением: у
// каждого вида свой обработчик, и два из трёх тестами не исполнялись.
func TestSyncSSEAndNDJSONMatchJSON(t *testing.T) {
	srv, caps, _, _ := setupAPI(t)
	ownerTok, memberTok, circleID, _ := circleWithMember(t, srv, caps)

	for i := range 3 {
		rec := doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+circleID+"/posts", ownerTok, map[string]any{
			"body": "запись " + strconv.Itoa(i), "entry_date": "2026-08-30",
		})
		if rec.Code != http.StatusCreated {
			t.Fatalf("запись %d: %d %s", i, rec.Code, rec.Body.String())
		}
	}

	// JSON — образец.
	rec := doGET(t, srv, "/api/v1/sync?cursor=0", memberTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("sync json: %d %s", rec.Code, rec.Body.String())
	}
	var jsonOut struct {
		Events []map[string]any `json:"events"`
		MaxSeq int64            `json:"max_seq"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &jsonOut); err != nil {
		t.Fatal(err)
	}
	if len(jsonOut.Events) == 0 {
		t.Fatal("в потоке нет событий")
	}

	seqs := func(events []map[string]any) []int64 {
		out := make([]int64, 0, len(events))
		for _, ev := range events {
			v, _ := ev["seq"].(float64)
			out = append(out, int64(v))
		}
		return out
	}
	want := seqs(jsonOut.Events)

	// NDJSON — строка на событие, поток закрывается сам.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sync?cursor=0", nil)
	req.Header.Set("Authorization", "Bearer "+memberTok)
	req.Header.Set("Accept", "application/x-ndjson")
	ndrec := httptest.NewRecorder()
	srv.ServeHTTP(ndrec, req)
	if ct := ndrec.Header().Get("Content-Type"); ct != "application/x-ndjson" {
		t.Fatalf("Content-Type NDJSON: %q", ct)
	}
	var ndEvents []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(ndrec.Body.String()), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var ev map[string]any
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			t.Fatalf("строка NDJSON %q: %v", line, err)
		}
		ndEvents = append(ndEvents, ev)
	}
	if got := seqs(ndEvents); !equalSeqs(got, want) {
		t.Fatalf("NDJSON отдал %v, JSON — %v", got, want)
	}

	// SSE — кадры event: sync, читаем пока не наберём все и закрываем поток.
	ctx, cancel := context.WithCancel(t.Context())
	sreq := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/sync?cursor=0", nil)
	sreq.Header.Set("Authorization", "Bearer "+memberTok)
	sreq.Header.Set("Accept", "text/event-stream")
	srec := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		srv.ServeHTTP(srec, sreq)
		close(done)
	}()
	deadline := time.After(3 * time.Second)
	for {
		if strings.Count(srec.Body.String(), "event: sync") >= len(want) {
			break
		}
		select {
		case <-deadline:
			cancel()
			<-done
			t.Fatalf("SSE отдал %d кадров из %d: %s", strings.Count(srec.Body.String(), "event: sync"), len(want), srec.Body.String())
		case <-time.After(20 * time.Millisecond):
		}
	}
	cancel()
	<-done

	var sseEvents []map[string]any
	for _, frame := range strings.Split(srec.Body.String(), "\n\n") {
		// Кадр hello несёт max_seq, а не событие (BKP-3).
		if strings.HasPrefix(frame, "event: hello") {
			continue
		}
		_, data, ok := strings.Cut(frame, "data: ")
		if !ok {
			continue
		}
		var ev map[string]any
		if err := json.Unmarshal([]byte(strings.TrimSpace(data)), &ev); err != nil {
			t.Fatalf("кадр SSE %q: %v", data, err)
		}
		sseEvents = append(sseEvents, ev)
	}
	if got := seqs(sseEvents); !equalSeqs(got, want) {
		t.Fatalf("SSE отдал %v, JSON — %v", got, want)
	}

	// Тело события тоже одно и то же.
	if !sameEvent(jsonOut.Events[0], ndEvents[0]) || !sameEvent(jsonOut.Events[0], sseEvents[0]) {
		t.Fatalf("виды потока расходятся телом события:\njson=%v\nndjson=%v\nsse=%v",
			jsonOut.Events[0], ndEvents[0], sseEvents[0])
	}
}

func equalSeqs(a, b []int64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func sameEvent(a, b map[string]any) bool {
	ra, _ := json.Marshal(a)
	rb, _ := json.Marshal(b)
	return string(ra) == string(rb)
}

// Инвариант: HEAD загрузки отвечает принятым числом байт. Клиент после
// обрыва спрашивает им, с какого места продолжать, — ответ должен расти
// ровно на принятое и не врать про «дописанное».
func TestUploadStatusAfterPartialChunk(t *testing.T) {
	srv, caps, _, _ := setupAPI(t)
	token, _ := registerSession(t, srv, caps, "ana@example.com")

	payload := bytes.Repeat([]byte("x"), 1024)
	rec := doJSON(t, srv, http.MethodPost, "/api/v1/uploads", token, map[string]any{
		"expected_size": len(payload), "mime_type": "image/jpeg", "filename": "f.jpg",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("сессия загрузки: %d %s", rec.Code, rec.Body.String())
	}
	sessionID := jsonStr(t, rec, "id")
	head := func() *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(http.MethodHead, "/api/v1/uploads/"+sessionID, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		return rec
	}

	rec = head()
	if rec.Code != http.StatusOK {
		t.Fatalf("HEAD пустой сессии: %d", rec.Code)
	}
	if got := rec.Header().Get("Upload-Offset"); got != "0" {
		t.Fatalf("Upload-Offset до первого куска: %q", got)
	}

	// Первая половина.
	req := httptest.NewRequest(http.MethodPut, "/api/v1/uploads/"+sessionID, bytes.NewReader(payload[:512]))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Upload-Offset", "0")
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("первый кусок: %d %s", rec.Code, rec.Body.String())
	}
	if got := head().Header().Get("Upload-Offset"); got != "512" {
		t.Fatalf("Upload-Offset после половины: %q", got)
	}

	// Кусок не с той позиции не принимается и счётчик не двигает.
	req = httptest.NewRequest(http.MethodPut, "/api/v1/uploads/"+sessionID, bytes.NewReader(payload[512:]))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Upload-Offset", "0")
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code == http.StatusOK {
		t.Fatalf("кусок не с той позиции принят: %d %s", rec.Code, rec.Body.String())
	}
	if got := head().Header().Get("Upload-Offset"); got != "512" {
		t.Fatalf("отвергнутый кусок сдвинул счётчик: %q", got)
	}

	// Вторая половина с правильной позиции — загрузка завершается.
	req = httptest.NewRequest(http.MethodPut, "/api/v1/uploads/"+sessionID, bytes.NewReader(payload[512:]))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Upload-Offset", "512")
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("второй кусок: %d %s", rec.Code, rec.Body.String())
	}
	if got := head().Header().Get("Upload-Offset"); got != "1024" {
		t.Fatalf("Upload-Offset после всех кусков: %q", got)
	}
	sum := sha256.Sum256(payload)
	rec = doJSON(t, srv, http.MethodPost, "/api/v1/uploads/"+sessionID+"/complete", token, map[string]string{
		"sha256": hex.EncodeToString(sum[:]),
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("завершение: %d %s", rec.Code, rec.Body.String())
	}

	// Чужая сессия загрузки не видна.
	otherTok, _ := registerSession(t, srv, caps, "zoe@example.com")
	req = httptest.NewRequest(http.MethodHead, "/api/v1/uploads/"+sessionID, nil)
	req.Header.Set("Authorization", "Bearer "+otherTok)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code == http.StatusOK {
		t.Fatalf("чужая сессия загрузки видна: %d", rec.Code)
	}
}

// Инвариант: прочитанное не отматывается назад. Курсор ставят из двух
// вкладок и после офлайна, и меньшее значение не должно возвращать круг в
// «непрочитано».
func TestReadCursorNeverGoesBackwards(t *testing.T) {
	srv, caps, _, _ := setupAPI(t)
	ownerTok, memberTok, circleID, _ := circleWithMember(t, srv, caps)

	for i := range 3 {
		rec := doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+circleID+"/posts", ownerTok, map[string]any{
			"body": "запись " + strconv.Itoa(i), "entry_date": "2026-08-30",
		})
		if rec.Code != http.StatusCreated {
			t.Fatalf("запись %d: %d %s", i, rec.Code, rec.Body.String())
		}
	}
	rec := doGET(t, srv, "/api/v1/sync?cursor=0", memberTok)
	var out struct {
		MaxSeq int64 `json:"max_seq"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.MaxSeq < 3 {
		t.Fatalf("мало событий для проверки: max_seq=%d", out.MaxSeq)
	}

	cursor := func() float64 {
		t.Helper()
		rec := doGET(t, srv, "/api/v1/circles", memberTok)
		if rec.Code != http.StatusOK {
			t.Fatalf("круги: %d %s", rec.Code, rec.Body.String())
		}
		var list struct {
			Circles []map[string]any `json:"circles"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
			t.Fatal(err)
		}
		for _, c := range list.Circles {
			if c["id"] == circleID {
				v, _ := c["last_read_seq"].(float64)
				return v
			}
		}
		t.Fatalf("круг пропал из списка: %s", rec.Body.String())
		return 0
	}

	set := func(seq int64) int {
		t.Helper()
		rec := doJSON(t, srv, http.MethodPut, "/api/v1/circles/"+circleID+"/read_cursor", memberTok, map[string]any{
			"seq": seq,
		})
		return rec.Code
	}

	if code := set(out.MaxSeq); code != http.StatusOK {
		t.Fatalf("установка курсора: %d", code)
	}
	if got := cursor(); int64(got) != out.MaxSeq {
		t.Fatalf("курсор не записался: %v", got)
	}
	// Меньшее значение принимается ответом, но прочитанное не отматывает.
	if code := set(1); code != http.StatusOK {
		t.Fatalf("откат курсора: %d", code)
	}
	if got := cursor(); int64(got) != out.MaxSeq {
		t.Fatalf("курсор отмотался назад: %v, было %d", got, out.MaxSeq)
	}
	// Ноль не отматывает, отрицательное — отказ.
	if code := set(0); code != http.StatusOK {
		t.Fatalf("нулевой курсор: %d", code)
	}
	if code := set(-5); code != http.StatusBadRequest {
		t.Fatalf("отрицательный курсор: %d", code)
	}
	if got := cursor(); int64(got) != out.MaxSeq {
		t.Fatalf("курсор сброшен нулём: %v", got)
	}
}
