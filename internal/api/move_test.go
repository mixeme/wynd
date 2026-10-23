package api_test

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/api"
	"gitea.mixdep.ru/mix/wynd/internal/auth"
)

// moveCall — запрос нового сервера к прежнему: без входа, ключ ссылки в
// заголовке.
func moveCall(t *testing.T, srv *api.Server, method, path, link string, body any, rangeHeader string) *httptest.ResponseRecorder {
	t.Helper()
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		rd = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, rd)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if link != "" {
		req.Header.Set("Authorization", "Move "+link[strings.LastIndex(link, "/")+1:])
	}
	if rangeHeader != "" {
		req.Header.Set("Range", rangeHeader)
	}
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	return rec
}

type moveOwnerState struct {
	Estimate struct {
		Posts   int   `json:"posts"`
		Members int   `json:"members"`
		Bytes   int64 `json:"bytes"`
	} `json:"estimate"`
	FrozenAt string `json:"frozen_at"`
	Move     *struct {
		Status        string `json:"status"`
		LinkAlive     bool   `json:"link_alive"`
		ArchiveStatus string `json:"archive_status"`
		ArchiveBytes  int64  `json:"archive_bytes"`
		PulledBytes   int64  `json:"pulled_bytes"`
		DestURL       string `json:"dest_url"`
		AcceptedPosts int    `json:"accepted_posts"`
	} `json:"move"`
}

func readMoveState(t *testing.T, srv *api.Server, tok, circleID string) moveOwnerState {
	t.Helper()
	rec := doGET(t, srv, "/api/v1/circles/"+circleID+"/move", tok)
	if rec.Code != http.StatusOK {
		t.Fatalf("перенос: %d %s", rec.Code, rec.Body.String())
	}
	var st moveOwnerState
	if err := json.Unmarshal(rec.Body.Bytes(), &st); err != nil {
		t.Fatal(err)
	}
	return st
}

func startMove(t *testing.T, srv *api.Server, tok, circleID string) string {
	t.Helper()
	rec := doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+circleID+"/move", tok, nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("начать перенос: %d %s", rec.Code, rec.Body.String())
	}
	return jsonStr(t, rec, "link")
}

// archiveCounts — записей, имён и байт файлов в архиве: то, что новый сервер
// сообщает прежнему в «принято».
func archiveCounts(t *testing.T, file []byte) (posts, names int, media int64) {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(file), int64(len(file)))
	if err != nil {
		t.Fatal(err)
	}
	read := func(name string) (cols []string, rows [][]any) {
		f, err := zr.Open(name)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		var tbl struct {
			Columns []string `json:"columns"`
			Rows    [][]any  `json:"rows"`
		}
		if err := json.NewDecoder(f).Decode(&tbl); err != nil {
			t.Fatal(err)
		}
		return tbl.Columns, tbl.Rows
	}
	_, p := read("data/posts.json")
	_, n := read("data/identities.json")
	cols, blobs := read("data/blobs.json")
	for _, row := range blobs {
		for i, c := range cols {
			if c == "size_bytes" {
				media += int64(row[i].(float64))
			}
		}
	}
	return len(p), len(n), media
}

// Инвариант (план 53, 6.24 → 6.34 → 6.35): ссылка выдаётся один раз и круг
// замирает; новый сервер по ключу видит карточку, первым берёт перенос (второй
// адрес — отказ), забирает архив кусками; «принято» с чужими числами не
// принимается, со своими — круг стоит замершим с адресом нового сервера, и
// только «Завершить перенос» его удаляет.
func TestMoveSourceFlow(t *testing.T) {
	srv, tok, circleID := fullArchiveCircle(t)
	st := readMoveState(t, srv, tok, circleID)
	if st.Move != nil || st.Estimate.Posts != 2 || st.Estimate.Members != 1 {
		t.Fatalf("до переноса: %+v", st)
	}
	link := startMove(t, srv, tok, circleID)
	if !strings.Contains(link, "/move/") || len(link[strings.LastIndex(link, "/")+1:]) != 19 {
		t.Fatalf("ссылка: %q", link)
	}
	if rec := doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+circleID+"/move", tok, nil); rec.Code != http.StatusConflict {
		t.Fatalf("второй перенос: %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+circleID+"/posts", tok, map[string]any{
		"body": "после ссылки", "entry_date": "2026-10-15",
	}); rec.Code == http.StatusCreated {
		t.Fatal("в замерший круг записали")
	}
	for _, guard := range []string{"/full_archive/revive", "/transfer"} {
		if rec := doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+circleID+guard, tok, map[string]any{"new_owner_account_id": "x"}); rec.Code != http.StatusConflict {
			t.Fatalf("%s во время переноса: %d %s", guard, rec.Code, rec.Body.String())
		}
	}

	if rec := moveCall(t, srv, http.MethodGet, "/api/v1/move", "https://x/move/AAAA-BBBB-CCCC-DDDD", nil, ""); rec.Code != http.StatusNotFound {
		t.Fatalf("чужой ключ: %d", rec.Code)
	}
	rec := moveCall(t, srv, http.MethodGet, "/api/v1/move", link, nil, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("карточка: %d %s", rec.Code, rec.Body.String())
	}
	var card struct {
		Format string `json:"format"`
		Status string `json:"status"`
		Circle struct {
			Name  string `json:"name"`
			Posts int    `json:"posts"`
		} `json:"circle"`
		ServerName string `json:"server_name"`
		FrozenAt   string `json:"frozen_at"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &card); err != nil {
		t.Fatal(err)
	}
	if card.Format != "wynd-move" || card.Status != "waiting" || card.Circle.Name != "Семья" || card.Circle.Posts != 2 || card.ServerName != "Дом Ани" || card.FrozenAt == "" {
		t.Fatalf("карточка: %s", rec.Body.String())
	}

	if rec := moveCall(t, srv, http.MethodGet, "/api/v1/move/archive", link, nil, ""); rec.Code != http.StatusGone {
		t.Fatalf("архив до «беру»: %d %s", rec.Code, rec.Body.String())
	}
	claim := map[string]string{"dest_url": "https://wynd.u-slavy.ru/", "dest_name": "У Славы"}
	if rec := moveCall(t, srv, http.MethodPost, "/api/v1/move/claim", link, claim, ""); rec.Code != http.StatusOK {
		t.Fatalf("беру: %d %s", rec.Code, rec.Body.String())
	}
	if rec := moveCall(t, srv, http.MethodPost, "/api/v1/move/claim", link, map[string]string{"dest_url": "https://other.example"}, ""); rec.Code != http.StatusConflict {
		t.Fatalf("второй сервер: %d %s", rec.Code, rec.Body.String())
	}
	if rec := moveCall(t, srv, http.MethodPost, "/api/v1/move/claim", link, claim, ""); rec.Code != http.StatusOK {
		t.Fatalf("тот же сервер после обрыва: %d %s", rec.Code, rec.Body.String())
	}
	deadline := time.Now().Add(10 * time.Second)
	for st = readMoveState(t, srv, tok, circleID); st.Move.ArchiveStatus != "ready"; st = readMoveState(t, srv, tok, circleID) {
		if time.Now().After(deadline) {
			t.Fatalf("архив под перенос не собрался: %+v", st.Move)
		}
		time.Sleep(20 * time.Millisecond)
	}
	if st.Move.Status != "pulling" || st.Move.DestURL != "https://wynd.u-slavy.ru" || st.Move.ArchiveBytes <= 0 {
		t.Fatalf("забирает: %+v", st.Move)
	}

	// Кусками, как качает новый сервер: ход растёт.
	half := st.Move.ArchiveBytes / 2
	first := moveCall(t, srv, http.MethodGet, "/api/v1/move/archive", link, nil, "bytes=0-"+strconv.FormatInt(half-1, 10))
	if first.Code != http.StatusPartialContent {
		t.Fatalf("первый кусок: %d %s", first.Code, first.Body.String())
	}
	if got := readMoveState(t, srv, tok, circleID).Move.PulledBytes; got != half {
		t.Fatalf("ход после первого куска: %d, ждали %d", got, half)
	}
	rest := moveCall(t, srv, http.MethodGet, "/api/v1/move/archive", link, nil, "bytes="+strconv.FormatInt(half, 10)+"-")
	if rest.Code != http.StatusPartialContent {
		t.Fatalf("остаток: %d", rest.Code)
	}
	file := append(first.Body.Bytes(), rest.Body.Bytes()...)
	if int64(len(file)) != st.Move.ArchiveBytes {
		t.Fatalf("длина: %d из %d", len(file), st.Move.ArchiveBytes)
	}
	posts, names, media := archiveCounts(t, file)
	if posts != 2 || names != 1 {
		t.Fatalf("в архиве: %d записей, %d имён", posts, names)
	}

	if rec := moveCall(t, srv, http.MethodPost, "/api/v1/move/accepted", link, map[string]any{"posts": posts - 1, "names": names, "bytes": media}, ""); rec.Code != http.StatusConflict {
		t.Fatalf("принято с чужими числами: %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+circleID+"/move/complete", tok, nil); rec.Code != http.StatusConflict {
		t.Fatalf("завершить до «принято»: %d %s", rec.Code, rec.Body.String())
	}
	if rec := moveCall(t, srv, http.MethodPost, "/api/v1/move/accepted", link, map[string]any{"posts": posts, "names": names, "bytes": media}, ""); rec.Code != http.StatusOK {
		t.Fatalf("принято: %d %s", rec.Code, rec.Body.String())
	}
	st = readMoveState(t, srv, tok, circleID)
	if st.Move.Status != "accepted" || st.Move.AcceptedPosts != 2 || st.FrozenAt == "" {
		t.Fatalf("после «принято»: %+v", st)
	}
	var moved int
	if err := srv.Chronicle.DB().QueryRow(`SELECT COUNT(*) FROM events WHERE circle_id = ? AND event_type = 'circle_moved' AND summary LIKE '%wynd.u-slavy.ru%'`, circleID).Scan(&moved); err != nil || moved != 1 {
		t.Fatalf("строка «перенесла круг на …»: %d %v", moved, err)
	}
	rec = doGET(t, srv, "/api/v1/circles", tok)
	if !strings.Contains(rec.Body.String(), `"dest_url":"https://wynd.u-slavy.ru"`) {
		t.Fatalf("строка круга 2.20 без адреса: %s", rec.Body.String())
	}
	if rec := doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+circleID+"/move/complete", tok, nil); rec.Code != http.StatusOK {
		t.Fatalf("завершить: %d %s", rec.Code, rec.Body.String())
	}
	if rec := doGET(t, srv, "/api/v1/circles/"+circleID, tok); rec.Code != http.StatusNotFound {
		t.Fatalf("круг после завершения: %d", rec.Code)
	}
}

// Инвариант (план 53, 6.25, 2.33): «Отменить перенос» гасит ключ и оживляет
// круг; ссылка с вышедшим сроком не берётся, карточка говорит «expired».
func TestMoveCancelAndExpiry(t *testing.T) {
	srv, tok, circleID := fullArchiveCircle(t)
	link := startMove(t, srv, tok, circleID)
	if rec := doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+circleID+"/move/cancel", tok, nil); rec.Code != http.StatusOK {
		t.Fatalf("отменить: %d %s", rec.Code, rec.Body.String())
	}
	if st := readMoveState(t, srv, tok, circleID); st.Move != nil || st.FrozenAt != "" {
		t.Fatalf("после отмены: %+v", st)
	}
	if rec := doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+circleID+"/posts", tok, map[string]any{
		"body": "снова пишем", "entry_date": "2026-10-15",
	}); rec.Code != http.StatusCreated {
		t.Fatalf("круг не ожил: %d %s", rec.Code, rec.Body.String())
	}
	if rec := moveCall(t, srv, http.MethodPost, "/api/v1/move/claim", link, map[string]string{"dest_url": "https://wynd.u-slavy.ru"}, ""); rec.Code != http.StatusGone {
		t.Fatalf("отменённая ссылка: %d %s", rec.Code, rec.Body.String())
	}

	link = startMove(t, srv, tok, circleID)
	if _, err := srv.Chronicle.DB().Exec(`UPDATE circle_moves SET expires_at = ? WHERE circle_id = ? AND status = 'waiting'`,
		time.Now().Add(-time.Minute).UTC().Format(time.RFC3339Nano), circleID); err != nil {
		t.Fatal(err)
	}
	rec := moveCall(t, srv, http.MethodGet, "/api/v1/move", link, nil, "")
	if rec.Code != http.StatusOK || jsonStr(t, rec, "status") != "expired" {
		t.Fatalf("карточка истёкшей: %d %s", rec.Code, rec.Body.String())
	}
	if rec := moveCall(t, srv, http.MethodPost, "/api/v1/move/claim", link, map[string]string{"dest_url": "https://wynd.u-slavy.ru"}, ""); rec.Code != http.StatusGone {
		t.Fatalf("истёкшая ссылка: %d %s", rec.Code, rec.Body.String())
	}
	if st := readMoveState(t, srv, tok, circleID); st.Move == nil || st.Move.LinkAlive || st.FrozenAt == "" {
		t.Fatalf("истёкший перенос держит круг замершим до отмены: %+v", st)
	}
}

// Инвариант (план 53, 2.18 → 7.25, 6.35): два сервера. Владелица на прежнем
// выдаёт ссылку; на новом её вставляет другой человек — сервер показывает
// карточку круга, забирает архив сам (кусками, после «ещё собирается»),
// поднимает круг со снимком и записью, владелец — тот, кто вставил ссылку, в
// ленте «Круг перенесён с …»; прежний сервер получил «принято и сверено».
func TestMoveBetweenServers(t *testing.T) {
	src, srcCaps, _, _ := setupAPI(t)
	anya, _ := registerSession(t, src, srcCaps, "anya@example.com")
	rec := doJSON(t, src, http.MethodPost, "/api/v1/circles", anya, map[string]any{"name": "Семья", "owner_name": "Аня"})
	circleID := jsonStr(t, rec, "id")
	inv, err := src.Auth.CreateInvite(t.Context(), auth.CreateInviteInput{CircleID: circleID, Kind: auth.InviteSingle, MaxUses: 1, TTL: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	acceptInvite(t, src, srcCaps, inv.Token, "bob@example.com", "Боб")
	photo := []byte(strings.Repeat("jpeg-bytes-", 1000))
	blobID := uploadBytes(t, src, anya, photo, "image/jpeg")
	if rec := doJSON(t, src, http.MethodPost, "/api/v1/circles/"+circleID+"/posts", anya, map[string]any{
		"body": "на даче", "entry_date": "2026-10-14",
		"media": []map[string]any{{"blob_id": blobID, "kind": "photo", "is_cover": true}},
	}); rec.Code != http.StatusCreated {
		t.Fatalf("запись: %d %s", rec.Code, rec.Body.String())
	}
	srcHTTP := httptest.NewServer(src)
	defer srcHTTP.Close()
	link := startMove(t, src, anya, circleID)
	link = srcHTTP.URL + "/move/" + link[strings.LastIndex(link, "/")+1:]

	dst, dstCaps, _, _ := setupAPI(t)
	dst.MoveRetry = 10 * time.Millisecond
	kot, _ := registerSession(t, dst, dstCaps, "kot@example.com")

	for link, want := range map[string]string{
		"https://example.org/not-a-move":                  "link_invalid",
		srcHTTP.URL + "/move/AAAA-BBBB-CCCC-DDDD":         "move_not_found",
		srcHTTP.URL + "/move/" + strings.Repeat("x", 300): "link_invalid",
	} {
		rec := doJSON(t, dst, http.MethodPost, "/api/v1/moves/incoming", kot, map[string]string{"link": link})
		if rec.Code != http.StatusBadRequest || jsonStr(t, rec, "error") != want {
			t.Fatalf("%s: %d %s, ждали %s", link, rec.Code, rec.Body.String(), want)
		}
	}
	plain := httptest.NewServer(http.NotFoundHandler())
	defer plain.Close()
	if rec := doJSON(t, dst, http.MethodPost, "/api/v1/moves/incoming", kot, map[string]string{"link": plain.URL + "/move/AAAA-BBBB-CCCC-DDDD"}); jsonStr(t, rec, "error") != "source_too_old" {
		t.Fatalf("не Wynd: %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, dst, http.MethodPost, "/api/v1/moves/incoming", kot, map[string]string{"link": link})
	if rec.Code != http.StatusOK {
		t.Fatalf("ссылка: %d %s", rec.Code, rec.Body.String())
	}
	var card struct {
		ID         string `json:"id"`
		Status     string `json:"status"`
		SourceName string `json:"source_name"`
		LinkAlive  bool   `json:"link_alive"`
		Fits       bool   `json:"fits"`
		// «Аня — это вы?» (A12): почты Кота в круге нет.
		OwnerName     string `json:"owner_name"`
		ReviverAbsent bool   `json:"reviver_absent"`
		Circle     struct {
			Name    string `json:"name"`
			Posts   int    `json:"posts"`
			Members int    `json:"members"`
			Bytes   int64  `json:"bytes"`
		} `json:"circle"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &card); err != nil {
		t.Fatal(err)
	}
	if card.Status != "draft" || card.SourceName != "Дом Ани" || !card.LinkAlive || !card.Fits ||
		card.Circle.Name != "Семья" || card.Circle.Posts != 1 || card.Circle.Members != 2 || card.Circle.Bytes < int64(len(photo)) {
		t.Fatalf("карточка 2.18: %s", rec.Body.String())
	}
	if card.OwnerName != "Аня" || !card.ReviverAbsent {
		t.Fatalf("«Аня — это вы?»: %s", rec.Body.String())
	}
	if rec := doJSON(t, dst, http.MethodPost, "/api/v1/moves/incoming/"+card.ID+"/start", kot, map[string]any{
		"new_member": map[string]any{"name": " "},
	}); rec.Code != http.StatusBadRequest {
		t.Fatalf("без имени: %d %s", rec.Code, rec.Body.String())
	}
	// «Да, это я» — лицо прежней владелицы, как и до A12.
	if rec := doJSON(t, dst, http.MethodPost, "/api/v1/moves/incoming/"+card.ID+"/start", kot, map[string]any{"send_invites": true}); rec.Code != http.StatusOK {
		t.Fatalf("поднять: %d %s", rec.Code, rec.Body.String())
	}

	var got struct {
		Move struct {
			Status string `json:"status"`
			Error  string `json:"error"`
			Link   string `json:"link"`
			Result struct {
				CircleID string `json:"circle_id"`
				Posts    int    `json:"posts"`
				Names    int    `json:"names"`
				Bytes    int64  `json:"bytes"`
				Verified bool   `json:"verified"`
				Invited  int    `json:"invited"`
				Here     *int   `json:"already_here"`
			} `json:"result"`
		} `json:"move"`
	}
	deadline := time.Now().Add(15 * time.Second)
	for {
		rec := doGET(t, dst, "/api/v1/moves/incoming", kot)
		got.Move.Link = ""
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if got.Move.Status == "done" || got.Move.Status == "failed" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("перенос не закончился: %s", rec.Body.String())
		}
		time.Sleep(20 * time.Millisecond)
	}
	r := got.Move.Result
	if got.Move.Status != "done" || !r.Verified || r.Posts != 1 || r.Names != 2 || r.Bytes <= int64(len(photo)) || got.Move.Link != "" {
		t.Fatalf("итог 2.19: %+v", got.Move)
	}
	// Итог — сколько круг занимает по квоте: фото и текст (C42).
	var quota struct {
		Used int64 `json:"used_bytes"`
	}
	_ = json.Unmarshal(doGET(t, dst, "/api/v1/circles/"+r.CircleID+"/quota", kot).Body.Bytes(), &quota)
	if r.Bytes != quota.Used {
		t.Fatalf("итог переноса %d байт, по квоте %d", r.Bytes, quota.Used)
	}
	// Кот вставил ссылку и получил лицо прежней владелицы (план 52); Боб здесь
	// не бывал — ему письмо.
	if r.Invited != 1 {
		t.Fatalf("приглашения: %d", r.Invited)
	}
	// «Уже здесь» в итоге (C51): кроме Кота, на новом сервере никого.
	if r.Here == nil || *r.Here != 0 {
		t.Fatalf("already_here: %v", r.Here)
	}

	// Круг на новом сервере — у Кота, со снимком и строкой о переносе.
	rec = doGET(t, dst, "/api/v1/circles/"+r.CircleID+"/feed", kot)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "на даче") {
		t.Fatalf("лента нового круга: %d %s", rec.Code, rec.Body.String())
	}
	host := strings.TrimPrefix(srcHTTP.URL, "http://")
	var movedIn int
	if err := dst.Chronicle.DB().QueryRow(`SELECT COUNT(*) FROM events WHERE circle_id = ? AND event_type = 'circle_moved_in' AND summary = ?`,
		r.CircleID, "Круг перенесён с "+host).Scan(&movedIn); err != nil || movedIn != 1 {
		t.Fatalf("строка «Круг перенесён с %s»: %d %v", host, movedIn, err)
	}
	if rec := doJSON(t, dst, http.MethodPost, "/api/v1/circles/"+r.CircleID+"/posts", kot, map[string]any{
		"body": "уже здесь", "entry_date": "2026-10-15",
	}); rec.Code != http.StatusCreated {
		t.Fatalf("писать в перенесённый круг: %d %s", rec.Code, rec.Body.String())
	}

	// Прежний сервер: «принято и сверено», адрес нового — из обмена.
	deadline = time.Now().Add(5 * time.Second)
	st := readMoveState(t, src, anya, circleID)
	for st.Move.Status != "accepted" && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
		st = readMoveState(t, src, anya, circleID)
	}
	if st.Move.Status != "accepted" || st.Move.AcceptedPosts != 1 || st.Move.DestURL != strings.TrimRight(dst.PublicURL(), "/") {
		t.Fatalf("прежний сервер: %+v", st.Move)
	}
}
