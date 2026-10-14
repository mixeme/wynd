package api_test

import (
	"archive/zip"
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/api"
	"gitea.mixdep.ru/mix/wynd/internal/archive"
	"gitea.mixdep.ru/mix/wynd/internal/auth"
)

type fullState struct {
	EstimateBytes int64 `json:"estimate_bytes"`
	Posts         int   `json:"posts"`
	Fits          bool  `json:"fits"`
	Archive       *struct {
		ID            string `json:"id"`
		Status        string `json:"status"`
		SizeBytes     int64  `json:"size_bytes"`
		RemovedReason string `json:"removed_reason"`
		Encrypted     bool   `json:"encrypted"`
	} `json:"archive"`
}

func fullArchiveCircle(t *testing.T) (*api.Server, string, string) {
	t.Helper()
	srv, caps, _, _ := setupAPI(t)
	tok, _ := registerSession(t, srv, caps, "anya@example.com")
	rec := doJSON(t, srv, http.MethodPost, "/api/v1/circles", tok, map[string]any{"name": "Семья", "owner_name": "Аня"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("круг: %d %s", rec.Code, rec.Body.String())
	}
	circleID := jsonStr(t, rec, "id")
	for _, body := range []string{"Мы на даче", "Яблони зацвели"} {
		if rec := doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+circleID+"/posts", tok, map[string]any{
			"body": body, "entry_date": "2026-10-14",
		}); rec.Code != http.StatusCreated {
			t.Fatalf("запись: %d %s", rec.Code, rec.Body.String())
		}
	}
	return srv, tok, circleID
}

func readFullState(t *testing.T, srv *api.Server, tok, circleID string) fullState {
	t.Helper()
	rec := doGET(t, srv, "/api/v1/circles/"+circleID+"/full_archive", tok)
	if rec.Code != http.StatusOK {
		t.Fatalf("состояние архива: %d %s", rec.Code, rec.Body.String())
	}
	var st fullState
	if err := json.Unmarshal(rec.Body.Bytes(), &st); err != nil {
		t.Fatal(err)
	}
	return st
}

func waitFullStatus(t *testing.T, srv *api.Server, tok, circleID, want string) fullState {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		st := readFullState(t, srv, tok, circleID)
		if st.Archive != nil && st.Archive.Status == want {
			return st
		}
		if time.Now().After(deadline) {
			t.Fatalf("архив не стал %q: %+v", want, st.Archive)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func fullLink(t *testing.T, srv *api.Server, tok, circleID string) string {
	t.Helper()
	rec := doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+circleID+"/full_archive/link", tok, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("ссылка: %d %s", rec.Code, rec.Body.String())
	}
	return jsonStr(t, rec, "url")
}

func fetch(srv *api.Server, url, rangeHeader string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, url, nil)
	if rangeHeader != "" {
		req.Header.Set("Range", rangeHeader)
	}
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	return rec
}

// Инвариант (план 51, 6.29 → 6.40 → 6.33): архив собирается в фоне, готовый
// файл — ZIP со всем кругом; оборванное скачивание файл не трогает, докачка
// по той же ссылке работает; дошёл последний байт — файл стёрт, место
// освободилось, круг можно архивировать снова.
func TestFullArchiveBuildDownloadAndResume(t *testing.T) {
	srv, tok, circleID := fullArchiveCircle(t)
	st := readFullState(t, srv, tok, circleID)
	if st.Archive != nil || st.Posts != 2 || !st.Fits || st.EstimateBytes <= 0 {
		t.Fatalf("до архива: %+v", st)
	}
	rec := doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+circleID+"/full_archive", tok, map[string]any{})
	if rec.Code != http.StatusCreated {
		t.Fatalf("собрать: %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+circleID+"/full_archive", tok, map[string]any{}); rec.Code != http.StatusConflict {
		t.Fatalf("второй архив, пока первый не скачан: %d %s", rec.Code, rec.Body.String())
	}
	st = waitFullStatus(t, srv, tok, circleID, "ready")
	url := fullLink(t, srv, tok, circleID)

	head := fetch(srv, url, "bytes=0-99")
	if head.Code != http.StatusPartialContent || head.Body.Len() != 100 {
		t.Fatalf("начало файла: %d, %d байт", head.Code, head.Body.Len())
	}
	if st := readFullState(t, srv, tok, circleID); st.Archive.Status != "ready" {
		t.Fatalf("оборванное скачивание стёрло файл: %+v", st.Archive)
	}
	tail := fetch(srv, url, "bytes=100-")
	if tail.Code != http.StatusPartialContent {
		t.Fatalf("докачка: %d", tail.Code)
	}
	whole := append(head.Body.Bytes(), tail.Body.Bytes()...)
	if int64(len(whole)) != st.Archive.SizeBytes {
		t.Fatalf("файл %d байт, а в состоянии — %d", len(whole), st.Archive.SizeBytes)
	}
	zr, err := zip.NewReader(bytes.NewReader(whole), int64(len(whole)))
	if err != nil {
		t.Fatalf("не ZIP: %v", err)
	}
	names := map[string]bool{}
	for _, f := range zr.File {
		names[f.Name] = true
	}
	for _, want := range []string{"manifest.json", "data/posts.json", "data/accounts.json", "index.html"} {
		if !names[want] {
			t.Fatalf("в архиве нет %s: %v", want, names)
		}
	}
	st = readFullState(t, srv, tok, circleID)
	if st.Archive.Status != "removed" || st.Archive.RemovedReason != "downloaded" {
		t.Fatalf("после скачивания: %+v", st.Archive)
	}
	if rec := fetch(srv, url, ""); rec.Code != http.StatusNotFound {
		t.Fatalf("скачанный файл отдаётся снова: %d", rec.Code)
	}
	if entries, _ := os.ReadDir(filepath.Join(srv.DataDir, "archives")); len(entries) != 0 {
		t.Fatalf("файл остался на диске: %v", entries)
	}
	if rec := doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+circleID+"/full_archive", tok, map[string]any{}); rec.Code != http.StatusCreated {
		t.Fatalf("новый архив после скачанного: %d %s", rec.Code, rec.Body.String())
	}
}

// «Удалить архив, не скачивая» (6.33): файл стёрт, место освободилось.
func TestFullArchiveRemoveByOwner(t *testing.T) {
	srv, tok, circleID := fullArchiveCircle(t)
	doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+circleID+"/full_archive", tok, map[string]any{})
	waitFullStatus(t, srv, tok, circleID, "ready")
	if rec := doJSON(t, srv, http.MethodDelete, "/api/v1/circles/"+circleID+"/full_archive", tok, nil); rec.Code != http.StatusOK {
		t.Fatalf("удалить: %d %s", rec.Code, rec.Body.String())
	}
	st := readFullState(t, srv, tok, circleID)
	if st.Archive.Status != "removed" || st.Archive.RemovedReason != "owner" {
		t.Fatalf("после удаления: %+v", st.Archive)
	}
	if entries, _ := os.ReadDir(filepath.Join(srv.DataDir, "archives")); len(entries) != 0 {
		t.Fatalf("файл остался: %v", entries)
	}
}

// Готовый файл положить некуда (6.43): сборка — отказ no_room, а поток
// собирает архив прямо в ответ.
func TestFullArchiveNoRoomStreams(t *testing.T) {
	srv, tok, circleID := fullArchiveCircle(t)
	admin := adminToken(t, srv)
	if rec := doJSON(t, srv, http.MethodPut, "/api/v1/admin/storage/quota", admin, map[string]any{"quota_bytes": 1}); rec.Code != http.StatusOK {
		t.Fatalf("потолок: %d %s", rec.Code, rec.Body.String())
	}
	if st := readFullState(t, srv, tok, circleID); st.Fits {
		t.Fatalf("архив «помещается» на заполненный сервер: %+v", st)
	}
	if rec := doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+circleID+"/full_archive", tok, map[string]any{}); rec.Code != http.StatusInsufficientStorage {
		t.Fatalf("сборка без места: %d %s", rec.Code, rec.Body.String())
	}
	rec := doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+circleID+"/full_archive/stream_link", tok, map[string]any{})
	if rec.Code != http.StatusOK {
		t.Fatalf("ссылка на поток: %d %s", rec.Code, rec.Body.String())
	}
	url := jsonStr(t, rec, "url")
	got := fetch(srv, url, "")
	if got.Code != http.StatusOK || got.Header().Get("Content-Length") == "" {
		t.Fatalf("поток: %d", got.Code)
	}
	if _, err := zip.NewReader(bytes.NewReader(got.Body.Bytes()), int64(got.Body.Len())); err != nil {
		t.Fatalf("поток — не ZIP: %v", err)
	}
	if again := fetch(srv, url, ""); again.Code != http.StatusNotFound {
		t.Fatalf("ссылка на поток сработала дважды: %d", again.Code)
	}
}

// Зашифрованный архив: сервер получает открытый ключ и соль, файл
// открывается паролем, из которого ключ выведен на устройстве.
func TestFullArchiveEncryptedWithDeviceKey(t *testing.T) {
	srv, tok, circleID := fullArchiveCircle(t)
	salt := make([]byte, 16)
	_, _ = rand.Read(salt)
	priv, err := archive.DeriveKey("дача-самовар-облако-17", salt, archive.DefaultLogN)
	if err != nil {
		t.Fatal(err)
	}
	key := map[string]any{
		"public_key": base64.StdEncoding.EncodeToString(priv.PublicKey().Bytes()),
		"salt":       base64.StdEncoding.EncodeToString(salt),
		"log_n":      archive.DefaultLogN,
	}
	if rec := doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+circleID+"/full_archive", tok, map[string]any{
		"key": map[string]any{"public_key": "AAAA", "salt": key["salt"], "log_n": archive.DefaultLogN},
	}); rec.Code != http.StatusBadRequest {
		t.Fatalf("негодный ключ: %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+circleID+"/full_archive", tok, map[string]any{"key": key}); rec.Code != http.StatusCreated {
		t.Fatalf("собрать: %d %s", rec.Code, rec.Body.String())
	}
	st := waitFullStatus(t, srv, tok, circleID, "ready")
	if !st.Archive.Encrypted {
		t.Fatalf("архив не помечен зашифрованным: %+v", st.Archive)
	}
	got := fetch(srv, fullLink(t, srv, tok, circleID), "")
	r, err := archive.Decrypt(bytes.NewReader(got.Body.Bytes()), "дача-самовар-облако-17")
	if err != nil {
		t.Fatal(err)
	}
	plain, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := zip.NewReader(bytes.NewReader(plain), int64(len(plain))); err != nil {
		t.Fatalf("внутри не ZIP: %v", err)
	}
}

// Срок вышел — файл стёрт (9.12).
func TestFullArchiveExpires(t *testing.T) {
	srv, tok, circleID := fullArchiveCircle(t)
	doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+circleID+"/full_archive", tok, map[string]any{})
	waitFullStatus(t, srv, tok, circleID, "ready")
	if n := srv.ExpireFullArchives(t.Context(), time.Now().Add(71*time.Hour)); n != 0 {
		t.Fatalf("стёрт раньше срока: %d", n)
	}
	if n := srv.ExpireFullArchives(t.Context(), time.Now().Add(73*time.Hour)); n != 1 {
		t.Fatalf("по сроку стёрто %d", n)
	}
	st := readFullState(t, srv, tok, circleID)
	if st.Archive.Status != "removed" || st.Archive.RemovedReason != "expired" {
		t.Fatalf("после срока: %+v", st.Archive)
	}
}

// Удалили круг — его готовый архив уходит с ним, и с диска тоже.
func TestFullArchiveGoesWithCircle(t *testing.T) {
	srv, tok, circleID := fullArchiveCircle(t)
	doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+circleID+"/full_archive", tok, map[string]any{})
	waitFullStatus(t, srv, tok, circleID, "ready")
	if rec := doJSON(t, srv, http.MethodDelete, "/api/v1/circles/"+circleID, tok, map[string]any{"name": "Семья"}); rec.Code != http.StatusOK {
		t.Fatalf("удалить круг: %d %s", rec.Code, rec.Body.String())
	}
	if entries, _ := os.ReadDir(filepath.Join(srv.DataDir, "archives")); len(entries) != 0 {
		t.Fatalf("архив удалённого круга остался: %v", entries)
	}
}

// Инвариант (план 51, 6.32 → 6.34 → 6.30, 6.36): архив с удалением круга
// замораживает его — писать нельзя никому, участник качает свой архив, в
// ленте «остановил круг»; убрать круг можно только после скачивания файла;
// «Отменить — оживить круг» возвращает запись.
func TestFullArchiveWithCircleRemoval(t *testing.T) {
	srv, caps, _, _ := setupAPI(t)
	owner, _ := registerSession(t, srv, caps, "anya@example.com")
	rec := doJSON(t, srv, http.MethodPost, "/api/v1/circles", owner, map[string]any{"name": "Семья", "owner_name": "Аня", "owner_gender": "f"})
	circleID := jsonStr(t, rec, "id")
	inv, err := srv.Auth.CreateInvite(t.Context(), auth.CreateInviteInput{CircleID: circleID, Kind: auth.InviteSingle, MaxUses: 1, TTL: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	bob := acceptInvite(t, srv, caps, inv.Token, "bob@example.com", "Боб")
	post := func(tok string) int {
		return doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+circleID+"/posts", tok, map[string]any{"body": "ещё запись", "entry_date": "2026-10-14"}).Code
	}
	if code := post(bob); code != http.StatusCreated {
		t.Fatalf("запись до заморозки: %d", code)
	}

	if rec := doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+circleID+"/full_archive", owner, map[string]any{"delete_circle": true}); rec.Code != http.StatusCreated {
		t.Fatalf("архив с удалением: %d %s", rec.Code, rec.Body.String())
	}
	if code := post(bob); code != http.StatusForbidden {
		t.Fatalf("участник пишет в замерший круг: %d", code)
	}
	if code := post(owner); code != http.StatusForbidden {
		t.Fatalf("владелец пишет в замерший круг: %d", code)
	}
	var list struct {
		Circles []map[string]any `json:"circles"`
	}
	_ = json.Unmarshal(doGET(t, srv, "/api/v1/circles", bob).Body.Bytes(), &list)
	if len(list.Circles) != 1 || list.Circles[0]["frozen_at"] == nil || list.Circles[0]["owner_name"] != "Аня" {
		t.Fatalf("строка круга у участника: %+v", list.Circles)
	}
	if !eventsContain(syncEvents(t, srv, bob), "Аня остановила круг") {
		t.Fatal("в ленте нет «Аня остановила круг»")
	}
	// Свой архив участник качает и без цикла архивации.
	if rec := doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+circleID+"/archive/link", bob, nil); rec.Code != http.StatusOK {
		t.Fatalf("архив участника в замершем круге: %d %s", rec.Code, rec.Body.String())
	}

	waitFullStatus(t, srv, owner, circleID, "ready")
	if rec := doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+circleID+"/full_archive/remove_circle", owner, nil); rec.Code != http.StatusConflict {
		t.Fatalf("убрать круг до скачивания файла: %d %s", rec.Code, rec.Body.String())
	}
	if got := fetch(srv, fullLink(t, srv, owner, circleID), ""); got.Code != http.StatusOK {
		t.Fatalf("скачать: %d", got.Code)
	}
	if rec := doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+circleID+"/full_archive/remove_circle", owner, nil); rec.Code != http.StatusOK {
		t.Fatalf("убрать круг после скачивания: %d %s", rec.Code, rec.Body.String())
	}
	if rec := doGET(t, srv, "/api/v1/circles/"+circleID, bob); rec.Code != http.StatusNotFound {
		t.Fatalf("круг остался у участника: %d", rec.Code)
	}
}

// «Отменить — оживить круг»: сборка отменена, круг снова пишет.
func TestFullArchiveReviveCircle(t *testing.T) {
	srv, tok, circleID := fullArchiveCircle(t)
	doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+circleID+"/full_archive", tok, map[string]any{"delete_circle": true})
	waitFullStatus(t, srv, tok, circleID, "ready")
	if rec := doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+circleID+"/full_archive/revive", tok, nil); rec.Code != http.StatusOK {
		t.Fatalf("оживить: %d %s", rec.Code, rec.Body.String())
	}
	st := readFullState(t, srv, tok, circleID)
	if st.Archive.Status != "removed" {
		t.Fatalf("архив после оживления: %+v", st.Archive)
	}
	if rec := doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+circleID+"/posts", tok, map[string]any{"body": "снова пишем", "entry_date": "2026-10-15"}); rec.Code != http.StatusCreated {
		t.Fatalf("запись после оживления: %d %s", rec.Code, rec.Body.String())
	}
}

// Панель (9.12): готовые архивы видны с размером, «Удалить все» стирает их,
// а владелец видит, что файл удалил администратор (6.41); срок меняется.
func TestAdminClearsReadyArchives(t *testing.T) {
	srv, tok, circleID := fullArchiveCircle(t)
	admin := adminToken(t, srv)
	doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+circleID+"/full_archive", tok, map[string]any{})
	st := waitFullStatus(t, srv, tok, circleID, "ready")
	var storage struct {
		ArchivesBytes int64 `json:"archives_bytes"`
		ArchivesCount int   `json:"archives_count"`
		Retention     int   `json:"archive_retention_hours"`
		UsedBytes     int64 `json:"used_bytes"`
	}
	_ = json.Unmarshal(doGET(t, srv, "/api/v1/admin/storage", admin).Body.Bytes(), &storage)
	if storage.ArchivesCount != 1 || storage.ArchivesBytes != st.Archive.SizeBytes || storage.Retention != 72 || storage.UsedBytes < storage.ArchivesBytes {
		t.Fatalf("хранилище: %+v, архив %d байт", storage, st.Archive.SizeBytes)
	}
	if rec := doJSON(t, srv, http.MethodPut, "/api/v1/admin/storage/archive_retention", admin, map[string]any{"hours": 0}); rec.Code != http.StatusBadRequest {
		t.Fatalf("срок 0 часов: %d", rec.Code)
	}
	if rec := doJSON(t, srv, http.MethodPut, "/api/v1/admin/storage/archive_retention", admin, map[string]any{"hours": 24}); rec.Code != http.StatusOK {
		t.Fatalf("срок: %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, srv, http.MethodPost, "/api/v1/admin/storage/archives/clear", admin, nil); rec.Code != http.StatusOK {
		t.Fatalf("удалить все: %d %s", rec.Code, rec.Body.String())
	}
	st = readFullState(t, srv, tok, circleID)
	if st.Archive.Status != "removed" || st.Archive.RemovedReason != "admin" || st.Archive == nil {
		t.Fatalf("после очистки: %+v", st.Archive)
	}
	_ = json.Unmarshal(doGET(t, srv, "/api/v1/admin/storage", admin).Body.Bytes(), &storage)
	if storage.ArchivesCount != 0 || storage.ArchivesBytes != 0 || storage.Retention != 24 {
		t.Fatalf("хранилище после очистки: %+v", storage)
	}
}
