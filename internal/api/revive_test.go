package api_test

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/auth"
	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
)

// Инвариант (план 52): полный архив, собранный сервером, оживает у другого
// человека на этом же сервере — файлы загружаются обычными загрузками, круг
// встаёт рядом с исходным с той же записью и снимком; ожививший — владелец;
// участник, которого на сервере знают по почте, — в своём имени.
func TestReviveCircleFromArchiveHTTP(t *testing.T) {
	srv, caps, _, _ := setupAPI(t)
	anya, _ := registerSession(t, srv, caps, "anya@example.com")
	rec := doJSON(t, srv, http.MethodPost, "/api/v1/circles", anya, map[string]any{"name": "Семья", "owner_name": "Аня"})
	circleID := jsonStr(t, rec, "id")
	inv, err := srv.Auth.CreateInvite(t.Context(), auth.CreateInviteInput{CircleID: circleID, Kind: auth.InviteSingle, MaxUses: 1, TTL: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	bob := acceptInvite(t, srv, caps, inv.Token, "bob@example.com", "Боб")
	photo := []byte(strings.Repeat("jpeg-bytes-", 1000))
	blobID := uploadBytes(t, srv, anya, photo, "image/jpeg")
	if rec := doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+circleID+"/posts", anya, map[string]any{
		"body": "на даче", "entry_date": "2026-10-14",
		"media": []map[string]any{{"blob_id": blobID, "kind": "photo", "is_cover": true}},
	}); rec.Code != http.StatusCreated {
		t.Fatalf("запись: %d %s", rec.Code, rec.Body.String())
	}

	// Архив — как его скачает владелица.
	doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+circleID+"/full_archive", anya, map[string]any{})
	waitFullStatus(t, srv, anya, circleID, "ready")
	file := fetch(srv, fullLink(t, srv, anya, circleID), "").Body.Bytes()
	zr, err := zip.NewReader(bytes.NewReader(file), int64(len(file)))
	if err != nil {
		t.Fatal(err)
	}
	read := func(name string) []byte {
		f, err := zr.Open(name)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		defer f.Close()
		b, _ := io.ReadAll(f)
		return b
	}
	var manifest map[string]any
	_ = json.Unmarshal(read("manifest.json"), &manifest)
	tables := map[string]any{}
	for _, name := range manifest["tables"].([]any) {
		var t2 any
		_ = json.Unmarshal(read("data/"+name.(string)+".json"), &t2)
		tables[name.(string)] = t2
	}

	// Оживляет Слава: файлы — своими загрузками.
	slava, _ := registerSession(t, srv, caps, "slava@example.com")
	blobMap := map[string]string{}
	for _, f := range zr.File {
		if !strings.HasPrefix(f.Name, "media/") {
			continue
		}
		old := strings.TrimSuffix(strings.TrimPrefix(f.Name, "media/"), ".jpg")
		blobMap[old] = uploadBytes(t, srv, slava, read(f.Name), "image/jpeg")
	}
	if len(blobMap) != 1 {
		t.Fatalf("файлы в архиве: %v", blobMap)
	}
	// Чужие файлы чужим блобом не подменить.
	if rec := doJSON(t, srv, http.MethodPost, "/api/v1/revive", slava, map[string]any{
		"manifest": manifest, "tables": tables, "blob_map": map[string]string{blobID: blobID},
	}); rec.Code != http.StatusBadRequest {
		t.Fatalf("оживление чужим файлом: %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, srv, http.MethodPost, "/api/v1/revive", slava, map[string]any{
		"manifest": manifest, "tables": tables, "blob_map": blobMap,
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("оживить: %d %s", rec.Code, rec.Body.String())
	}
	var res struct {
		CircleID string `json:"circle_id"`
		Posts    int    `json:"posts"`
		Names    int    `json:"names"`
		Verified bool   `json:"verified"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &res)
	if res.CircleID == "" || res.CircleID == circleID || res.Posts != 1 || res.Names != 2 || !res.Verified {
		t.Fatalf("итог оживления: %s", rec.Body.String())
	}
	// Слава — владелец; Боб видит оживлённый круг рядом с исходным.
	if rec := doGET(t, srv, "/api/v1/circles/"+res.CircleID+"/quota", slava); rec.Code != http.StatusOK {
		t.Fatalf("ожививший не владелец: %d", rec.Code)
	}
	var list struct {
		Circles []map[string]any `json:"circles"`
	}
	_ = json.Unmarshal(doGET(t, srv, "/api/v1/circles", bob).Body.Bytes(), &list)
	if len(list.Circles) != 2 {
		t.Fatalf("у Боба круги: %+v", list.Circles)
	}
	if !eventsContain(syncEvents(t, srv, bob), "круг из архива") && !eventsContain(syncEvents(t, srv, bob), "Круг оживлён из архива") {
		t.Fatal("нет служебной строки оживления")
	}

	// Боб приехал с кругом — первый вход встречает его 1.10 («Круг оживили из
	// архива»: его запись на месте, вернулись оба); у Славы, который поднял
	// круг, встречи нет; «Войти в круг» её убирает.
	var detail struct {
		Arrival *struct {
			Kind          string   `json:"kind"`
			Returned      []string `json:"returned"`
			ReturnedCount int      `json:"returned_count"`
			Total         int      `json:"total"`
			YourPosts     int      `json:"your_posts"`
		} `json:"arrival"`
	}
	_ = json.Unmarshal(doGET(t, srv, "/api/v1/circles/"+res.CircleID, bob).Body.Bytes(), &detail)
	if a := detail.Arrival; a == nil || a.Kind != "revived" || a.Total != 2 || a.ReturnedCount != 2 || a.YourPosts != 0 || len(a.Returned) != 2 {
		t.Fatalf("встреча 1.9 у Боба: %+v", detail.Arrival)
	}
	detail.Arrival = nil
	_ = json.Unmarshal(doGET(t, srv, "/api/v1/circles/"+res.CircleID, slava).Body.Bytes(), &detail)
	if detail.Arrival != nil {
		t.Fatalf("встреча у поднявшего круг: %+v", detail.Arrival)
	}
	if rec := doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+res.CircleID+"/arrival/seen", bob, nil); rec.Code != http.StatusOK {
		t.Fatalf("войти в круг: %d %s", rec.Code, rec.Body.String())
	}
	_ = json.Unmarshal(doGET(t, srv, "/api/v1/circles/"+res.CircleID, bob).Body.Bytes(), &detail)
	if detail.Arrival != nil {
		t.Fatalf("встреча после «Войти в круг»: %+v", detail.Arrival)
	}
}

// Новая миграция — новая схема архивов: оживление обязано её знать, иначе
// архив, собранный этим же сервером, здесь не оживить.
func TestReviveKnowsCurrentSchema(t *testing.T) {
	srv, _, ch, _ := setupAPI(t)
	var v int
	if err := ch.DB().QueryRowContext(t.Context(), `SELECT MAX(version) FROM schema_migrations`).Scan(&v); err != nil {
		t.Fatal(err)
	}
	_ = srv
	if !slices.Contains(chronicle.ReviveSchemaVersions, v) {
		t.Fatalf("схема %d не в chronicle.ReviveSchemaVersions %v: проверить, менялись ли таблицы круга, и добавить", v, chronicle.ReviveSchemaVersions)
	}
}

// Место для круга, которого пока нет (7.30 → 7.31): не помещается — просьба
// встаёт в очередь панели с именем круга; дали — круг оживает со своей квотой.
func TestRevivePlaceRequest(t *testing.T) {
	srv, caps, _, _ := setupAPI(t)
	admin := adminToken(t, srv)
	slava, _ := registerSession(t, srv, caps, "slava@example.com")
	photo := []byte(strings.Repeat("x", 5000))
	blobID := uploadBytes(t, srv, slava, photo, "image/jpeg")
	if rec := doJSON(t, srv, http.MethodPut, "/api/v1/admin/storage/default_quota", admin, map[string]any{"default_circle_quota_bytes": 1000}); rec.Code != http.StatusOK {
		t.Fatalf("квота по умолчанию: %d %s", rec.Code, rec.Body.String())
	}
	room := func() map[string]any {
		var out map[string]any
		_ = json.Unmarshal(doGET(t, srv, "/api/v1/revive/room?bytes=5000", slava).Body.Bytes(), &out)
		return out
	}
	if r := room(); r["fits"] != false || r["room_bytes"].(float64) != 1000 {
		t.Fatalf("место до просьбы: %+v", r)
	}
	// Архив из одного круга с одним файлом — собран руками, как его прислало бы устройство.
	tables := map[string]any{
		"circles":    map[string]any{"columns": []string{"id", "name", "owner_account_id", "created_at", "updated_at", "color"}, "rows": [][]any{{"00000000-0000-7000-8000-00000000c001", "Дача", "old-owner", "2026-01-01T00:00:00Z", "2026-01-01T00:00:00Z", "ochre"}}},
		"identities": map[string]any{"columns": []string{"id", "circle_id", "account_id", "created_at"}, "rows": [][]any{{"00000000-0000-7000-8000-00000000d001", "00000000-0000-7000-8000-00000000c001", "old-owner", "2026-01-01T00:00:00Z"}}},
		"identity_names": map[string]any{"columns": []string{"id", "identity_id", "name", "effective_at"}, "rows": [][]any{{"00000000-0000-7000-8000-00000000e001", "00000000-0000-7000-8000-00000000d001", "Слава", "2026-01-01T00:00:00Z"}}},
		"memberships": map[string]any{"columns": []string{"id", "circle_id", "account_id", "identity_id", "status", "created_at", "updated_at"}, "rows": [][]any{{"00000000-0000-7000-8000-00000000f001", "00000000-0000-7000-8000-00000000c001", "old-owner", "00000000-0000-7000-8000-00000000d001", "active", "2026-01-01T00:00:00Z", "2026-01-01T00:00:00Z"}}},
		"membership_spans": map[string]any{"columns": []string{"id", "membership_id", "started_at"}, "rows": [][]any{{"00000000-0000-7000-8000-00000000f101", "00000000-0000-7000-8000-00000000f001", "2026-01-01T00:00:00Z"}}},
		"accounts": map[string]any{"columns": []string{"id", "email"}, "rows": [][]any{{"old-owner", "slava@example.com"}}},
		"blobs":    map[string]any{"columns": []string{"id", "sha256", "size_bytes"}, "rows": [][]any{{"00000000-0000-7000-8000-0000000000b1", sha256Hex(photo), len(photo)}}},
	}
	manifest := map[string]any{"format": "wynd-circle", "format_version": 1, "schema_version": 35, "posts": 0}
	revive := func() int {
		return doJSON(t, srv, http.MethodPost, "/api/v1/revive", slava, map[string]any{
			"manifest": manifest, "tables": tables, "blob_map": map[string]string{"00000000-0000-7000-8000-0000000000b1": blobID},
		}).Code
	}
	if code := revive(); code != http.StatusInsufficientStorage {
		t.Fatalf("оживление больше квоты: %d", code)
	}
	if rec := doJSON(t, srv, http.MethodPost, "/api/v1/revive/place_request", slava, map[string]any{"bytes": 6000, "circle_name": "Дача"}); rec.Code != http.StatusCreated {
		t.Fatalf("просьба: %d %s", rec.Code, rec.Body.String())
	}
	if r := room(); r["request"].(map[string]any)["status"] != "pending" {
		t.Fatalf("просьба на экране: %+v", r)
	}
	var queue struct {
		Requests []map[string]any `json:"requests"`
	}
	_ = json.Unmarshal(doGET(t, srv, "/api/v1/admin/quota_requests", admin).Body.Bytes(), &queue)
	if len(queue.Requests) != 1 || queue.Requests[0]["circle_name"] != "Дача" || queue.Requests[0]["circle_id"] != "" {
		t.Fatalf("очередь панели: %+v", queue.Requests)
	}
	if rec := doJSON(t, srv, http.MethodPost, "/api/v1/admin/quota_requests/"+queue.Requests[0]["id"].(string)+"/approve", admin, nil); rec.Code != http.StatusOK {
		t.Fatalf("дать: %d %s", rec.Code, rec.Body.String())
	}
	if r := room(); r["fits"] != true {
		t.Fatalf("место после «дать»: %+v", r)
	}
	if code := revive(); code != http.StatusCreated {
		t.Fatalf("оживление после «дать»: %d", code)
	}
	if r := room(); r["fits"] != false || r["request"] != nil {
		t.Fatalf("просьба не израсходована: %+v", r)
	}
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// «Ещё не вошли» (6.6): участник из архива с незнакомой серверу почтой —
// в списке с not_joined, и владелец зовёт его письмом.
func TestReviveWaitingMembers(t *testing.T) {
	srv, caps, _, _ := setupAPI(t)
	slava, _ := registerSession(t, srv, caps, "slava@example.com")
	c, i1, i2, n1, n2, m1, m2 := "00000000-0000-7000-8000-00000000c001", "00000000-0000-7000-8000-00000000d001", "00000000-0000-7000-8000-00000000d002",
		"00000000-0000-7000-8000-00000000e001", "00000000-0000-7000-8000-00000000e002", "00000000-0000-7000-8000-00000000f001", "00000000-0000-7000-8000-00000000f002"
	ts := "2026-01-01T00:00:00Z"
	tables := map[string]any{
		"circles":        map[string]any{"columns": []string{"id", "name", "owner_account_id", "created_at", "updated_at", "color"}, "rows": [][]any{{c, "Дача", "a1", ts, ts, "ochre"}}},
		"identities":     map[string]any{"columns": []string{"id", "circle_id", "account_id", "created_at"}, "rows": [][]any{{i1, c, "a1", ts}, {i2, c, "a2", ts}}},
		"identity_names": map[string]any{"columns": []string{"id", "identity_id", "name", "effective_at"}, "rows": [][]any{{n1, i1, "Слава", ts}, {n2, i2, "Коля", ts}}},
		"memberships": map[string]any{"columns": []string{"id", "circle_id", "account_id", "identity_id", "status", "created_at", "updated_at"},
			"rows": [][]any{{m1, c, "a1", i1, "active", ts, ts}, {m2, c, "a2", i2, "active", ts, ts}}},
		"membership_spans": map[string]any{"columns": []string{"id", "membership_id", "started_at"},
			"rows": [][]any{{"00000000-0000-7000-8000-00000000f101", m1, ts}, {"00000000-0000-7000-8000-00000000f102", m2, ts}}},
		"accounts": map[string]any{"columns": []string{"id", "email"}, "rows": [][]any{{"a1", "slava@example.com"}, {"a2", "kolya@elsewhere.example"}}},
	}
	manifest := map[string]any{"format": "wynd-circle", "format_version": 1, "schema_version": 35, "posts": 0}
	rec := doJSON(t, srv, http.MethodPost, "/api/v1/revive", slava, map[string]any{"manifest": manifest, "tables": tables, "blob_map": map[string]string{}})
	if rec.Code != http.StatusCreated {
		t.Fatalf("оживить: %d %s", rec.Code, rec.Body.String())
	}
	circleID := jsonStr(t, rec, "circle_id")
	var members struct {
		Members []map[string]any `json:"members"`
	}
	_ = json.Unmarshal(doGET(t, srv, "/api/v1/circles/"+circleID+"/members", slava).Body.Bytes(), &members)
	waiting := 0
	for _, m := range members.Members {
		if m["not_joined"] == true {
			waiting++
			if m["name"] != "Коля" {
				t.Fatalf("не вошёл не тот: %+v", m)
			}
		}
	}
	if waiting != 1 {
		t.Fatalf("ещё не вошли: %+v", members.Members)
	}
	rec = doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+circleID+"/members/invite_waiting", slava, map[string]any{})
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"invited":1`) {
		t.Fatalf("позвать всех: %d %s", rec.Code, rec.Body.String())
	}
	// Сервер закрыт для новых — а Коля входит своей почтой из архива и
	// попадает в круг (план 46, A1).
	for _, mode := range []auth.RegistrationMode{auth.ModeInvite, auth.ModeClosed} {
		if err := srv.Auth.SetRegistrationMode(t.Context(), mode); err != nil {
			t.Fatal(err)
		}
		if rec := doJSON(t, srv, http.MethodPost, "/api/v1/auth/code", "", map[string]string{"email": "kolya@elsewhere.example"}); rec.Code >= 300 {
			t.Fatalf("%s: код для Коли: %d %s", mode, rec.Code, rec.Body.String())
		}
	}
	rec = doJSON(t, srv, http.MethodPost, "/api/v1/auth/verify", "", map[string]string{"email": "kolya@elsewhere.example", "code": caps.Last("kolya@elsewhere.example")})
	if rec.Code != http.StatusOK {
		t.Fatalf("Коля входит: %d %s", rec.Code, rec.Body.String())
	}
	kolya := jsonStr(t, rec, "token")
	var list struct {
		Circles []map[string]any `json:"circles"`
	}
	_ = json.Unmarshal(doGET(t, srv, "/api/v1/circles", kolya).Body.Bytes(), &list)
	if len(list.Circles) != 1 || list.Circles[0]["id"] != circleID {
		t.Fatalf("круг у Коли: %+v", list.Circles)
	}
}
