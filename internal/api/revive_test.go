package api_test

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/auth"
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
}
