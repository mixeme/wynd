package selfupdate

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

// Ход скачивания: последний отчёт — весь файл и его размер из Content-Length.
// Полоса обновления показывает эти числа, пока установка идёт.
func TestFetchReportsProgress(t *testing.T) {
	payload := bytes.Repeat([]byte("y"), 4096)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", strconv.Itoa(len(payload)))
		_, _ = w.Write(payload)
	}))
	defer srv.Close()

	var done, total int64
	body, err := fetch(context.Background(), srv.Client(), srv.URL, 1<<20, func(d, tt int64) {
		done, total = d, tt
	})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(body, payload) {
		t.Fatal("тело скачалось не целиком")
	}
	if done != int64(len(payload)) || total != int64(len(payload)) {
		t.Fatalf("ход: %d из %d, ждали %d из %d", done, total, len(payload), len(payload))
	}
}

// Без обратного вызова скачивание не меняется, а размер остаётся неизвестным,
// если сервер его не отдал.
func TestFetchWithoutProgress(t *testing.T) {
	payload := []byte("small")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(payload)
	}))
	defer srv.Close()

	body, err := fetch(context.Background(), srv.Client(), srv.URL, 1<<20, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(body, payload) {
		t.Fatal("тело скачалось не целиком")
	}
}
