package api_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

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
	maxSeq, _ := syncSSE(t, srv, memberTok, 0)
	if maxSeq < 3 {
		t.Fatalf("мало событий для проверки: max_seq=%d", maxSeq)
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

	if code := set(maxSeq); code != http.StatusOK {
		t.Fatalf("установка курсора: %d", code)
	}
	if got := cursor(); int64(got) != maxSeq {
		t.Fatalf("курсор не записался: %v", got)
	}
	// Меньшее значение принимается ответом, но прочитанное не отматывает.
	if code := set(1); code != http.StatusOK {
		t.Fatalf("откат курсора: %d", code)
	}
	if got := cursor(); int64(got) != maxSeq {
		t.Fatalf("курсор отмотался назад: %v, было %d", got, maxSeq)
	}
	// Ноль не отматывает, отрицательное — отказ.
	if code := set(0); code != http.StatusOK {
		t.Fatalf("нулевой курсор: %d", code)
	}
	if code := set(-5); code != http.StatusBadRequest {
		t.Fatalf("отрицательный курсор: %d", code)
	}
	if got := cursor(); int64(got) != maxSeq {
		t.Fatalf("курсор сброшен нулём: %v", got)
	}
}
