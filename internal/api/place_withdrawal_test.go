package api_test

import (
	"encoding/json"
	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
	"net/http"
	"testing"
)

// Отзывается конкретная собственная просьба, не «последняя у учётки».
func TestWithdrawPlaceRequestHTTP(t *testing.T) {
	srv, caps, _, blobs := setupAPI(t)
	owner, account := registerSession(t, srv, caps, "owner@example.com")
	other, _ := registerSession(t, srv, caps, "other@example.com")
	admin := adminToken(t, srv)
	if rec := doJSON(t, srv, http.MethodPut, "/api/v1/admin/storage/default_quota", admin, map[string]any{"default_circle_quota_bytes": 1000}); rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	create := func(key string) string {
		rec := doJSON(t, srv, http.MethodPost, "/api/v1/revive/place_request", owner, map[string]any{"bytes": 5000, "circle_name": "Семья", "operation_key": key})
		if rec.Code != 201 {
			t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
		}
		return jsonStr(t, rec, "id")
	}
	withdraw := func(token, id string, want int) {
		t.Helper()
		rec := doJSON(t, srv, http.MethodDelete, "/api/v1/revive/place_requests/"+id, token, nil)
		if rec.Code != want {
			t.Fatalf("withdraw: %d %s; want %d", rec.Code, rec.Body.String(), want)
		}
	}
	id := create("archive:first")
	withdraw("", id, 403)
	withdraw(other, id, 404)
	withdraw(owner, "missing", 404)
	if rec := doJSON(t, srv, http.MethodPost, "/api/v1/admin/quota_requests/"+id+"/approve", admin, nil); rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	withdraw(owner, id, 200)
	withdraw(owner, id, 200)
	var room map[string]any
	_ = json.Unmarshal(doGET(t, srv, "/api/v1/revive/room?bytes=5000", owner).Body.Bytes(), &room)
	if room["fits"] != false || room["request"] != nil {
		t.Fatalf("withdrawn approval: %+v", room)
	}
	// Историческое решение администратора сохранено.
	req, err := blobs.QuotaRequestByID(t.Context(), id)
	if err != nil || req == nil || req.Status != "approved" {
		t.Fatalf("decision: %+v %v", req, err)
	}
	newID := create("archive:second")
	withdraw(owner, id, 200)
	last, err := blobs.LastPlaceRequest(t.Context(), account)
	if err != nil || last == nil || last.ID != newID || last.OperationKey != "archive:second" {
		t.Fatalf("new request: %+v %v", last, err)
	}
	withdraw(owner, newID, 200)
	// После отзыва администратор уже не может дать место устаревшей просьбе.
	if rec := doJSON(t, srv, http.MethodPost, "/api/v1/admin/quota_requests/"+newID+"/approve", admin, nil); rec.Code != 404 {
		t.Fatalf("late approval: %d %s", rec.Code, rec.Body.String())
	}
	usedID := create("archive:used")
	if _, err := blobs.ResolveQuotaRequest(t.Context(), usedID, true, ""); err != nil {
		t.Fatal(err)
	}
	if err := blobs.UsePlaceRequest(t.Context(), usedID); err != nil {
		t.Fatal(err)
	}
	withdraw(owner, usedID, 404)
	// Просьба существующего круга не является просьбой для переноса/оживления.
	regular := create("archive:regular")
	circle, _, _, err := srv.Chronicle.CreateCircle(t.Context(), chronicle.CreateCircleInput{Name: "Другой круг", OwnerAccountID: account, OwnerName: "Кот", EditWindow: chronicle.UnlimitedWindow()})
	if err != nil {
		t.Fatal(err)
	}
	_, err = srv.Chronicle.DB().ExecContext(t.Context(), `UPDATE quota_requests SET circle_id = ? WHERE id = ?`, circle.ID, regular)
	if err != nil {
		t.Fatal(err)
	}
	withdraw(owner, regular, 404)
}
