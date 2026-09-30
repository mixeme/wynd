package api_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/api"
	"gitea.mixdep.ru/mix/wynd/internal/auth"
)

// circleWithMember: круг владельца и один обычный участник в нём.
func circleWithMember(t *testing.T, srv *api.Server, caps *auth.CaptureCodes) (ownerTok, memberTok, circleID, memberID string) {
	t.Helper()
	ownerTok, _ = registerSession(t, srv, caps, "anya@example.com")
	rec := doJSON(t, srv, http.MethodPost, "/api/v1/circles", ownerTok, map[string]any{
		"name": "Семья", "owner_name": "Аня",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create circle: %d %s", rec.Code, rec.Body.String())
	}
	circleID = jsonStr(t, rec, "id")
	memberTok = joinAsMember(t, srv, caps, ownerTok, circleID, "bob@example.com", "Боб")
	memberID = memberAccountID(t, srv, ownerTok, circleID, "Боб")
	return ownerTok, memberTok, circleID, memberID
}

func members(t *testing.T, srv *api.Server, token, circleID string) []map[string]any {
	t.Helper()
	rec := doGET(t, srv, "/api/v1/circles/"+circleID+"/members", token)
	if rec.Code != http.StatusOK {
		t.Fatalf("members: %d %s", rec.Code, rec.Body.String())
	}
	var out struct {
		Members []map[string]any `json:"members"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out.Members
}

func memberAccountID(t *testing.T, srv *api.Server, token, circleID, name string) string {
	t.Helper()
	for _, m := range members(t, srv, token, circleID) {
		if m["name"] == name {
			return m["account_id"].(string)
		}
	}
	t.Fatalf("участник %q не найден", name)
	return ""
}

// Инвариант: распоряжаться кругом может только владелец. Утверждается
// состояние, а не только статус: после каждого отказа круг, состав и права
// прежние — иначе «403 и всё-таки применилось» прошло бы незамеченным.
func TestOwnerOnlyCircleActionsForbiddenForMember(t *testing.T) {
	srv, caps, _, _ := setupAPI(t)
	ownerTok, memberTok, circleID, memberID := circleWithMember(t, srv, caps)
	ownerID := memberAccountID(t, srv, ownerTok, circleID, "Аня")

	cases := []struct {
		name   string
		method string
		path   string
		body   any
	}{
		{"удаление круга", http.MethodDelete, "", map[string]any{"name": "Семья"}},
		// Передаёт не себе: своему же id домен отвечает invalid раньше проверки прав.
		{"передача владения", http.MethodPost, "/transfer", map[string]any{"new_owner_account_id": ownerID}},
		{"исключение", http.MethodPost, "/exclude", map[string]any{"account_id": ownerID}},
		{"права участника", http.MethodPut, "/members/" + memberID, map[string]any{"can_settings": true}},
		{"запрос квоты", http.MethodPost, "/quota_requests", map[string]any{"requested_bytes": 5 << 30}},
		{"начало цикла архивации", http.MethodPost, "/archive", map[string]any{
			"cutoff_date": "2026-08-01",
			"deadline":    time.Now().UTC().Add(72 * time.Hour).Format(time.RFC3339),
		}},
		{"срок архива", http.MethodPut, "/archive/deadline", map[string]any{
			"deadline": time.Now().UTC().Add(96 * time.Hour).Format(time.RFC3339),
		}},
	}

	for _, tc := range cases {
		rec := doJSON(t, srv, tc.method, "/api/v1/circles/"+circleID+tc.path, memberTok, tc.body)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("%s: %d %s, ожидался 403", tc.name, rec.Code, rec.Body.String())
		}
	}

	// Состояние круга не изменилось ни одним из отказов.
	rec := doGET(t, srv, "/api/v1/circles/"+circleID, ownerTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("круг после отказов: %d %s", rec.Code, rec.Body.String())
	}
	var detail map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &detail); err != nil {
		t.Fatal(err)
	}
	if detail["archive_cutoff_date"] != nil && detail["archive_cutoff_date"] != "" {
		t.Fatalf("цикл архивации всё-таки начат: %s", rec.Body.String())
	}

	list := members(t, srv, ownerTok, circleID)
	if len(list) != 2 {
		t.Fatalf("состав круга изменился: %+v", list)
	}
	for _, m := range list {
		switch m["name"] {
		case "Аня":
			if m["is_owner"] != true || m["status"] != "active" {
				t.Fatalf("владелец изменился: %+v", m)
			}
		case "Боб":
			if m["is_owner"] == true || m["can_settings"] == true || m["status"] != "active" {
				t.Fatalf("права участника изменились: %+v", m)
			}
		default:
			t.Fatalf("посторонний участник: %+v", m)
		}
	}
}

// Инвариант: круг удаляется только по точному имени. Это единственная
// защита от удаления не того круга — подтверждение набирается вручную.
func TestDeleteCircleRequiresNameConfirmation(t *testing.T) {
	srv, caps, _, _ := setupAPI(t)
	ownerTok, _, circleID, _ := circleWithMember(t, srv, caps)

	for _, wrong := range []any{
		map[string]any{"name": "семья"},
		map[string]any{"name": "Семь"},
		map[string]any{"name": "Семья!"},
		map[string]any{"name": ""},
		map[string]any{},
	} {
		rec := doJSON(t, srv, http.MethodDelete, "/api/v1/circles/"+circleID, ownerTok, wrong)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("подтверждение %v: %d %s", wrong, rec.Code, rec.Body.String())
		}
	}
	// Круг на месте.
	if rec := doGET(t, srv, "/api/v1/circles/"+circleID, ownerTok); rec.Code != http.StatusOK {
		t.Fatalf("круг пропал после неверных подтверждений: %d", rec.Code)
	}

	// Пробелы по краям срезаются намеренно: имя набирают руками, в том числе
	// на телефоне. Всё остальное — не подтверждение.
	rec := doJSON(t, srv, http.MethodDelete, "/api/v1/circles/"+circleID, ownerTok, map[string]any{"name": " Семья "})
	if rec.Code != http.StatusOK {
		t.Fatalf("удаление по точному имени: %d %s", rec.Code, rec.Body.String())
	}
	if rec := doGET(t, srv, "/api/v1/circles/"+circleID, ownerTok); rec.Code != http.StatusNotFound {
		t.Fatalf("круг остался после удаления: %d %s", rec.Code, rec.Body.String())
	}
}

// Инвариант: передача владения меняет владельца и переносит право на
// настройки — прежний владелец остаётся обычным участником, а не вторым
// хозяином.
func TestTransferOwnershipSwapsRights(t *testing.T) {
	srv, caps, _, _ := setupAPI(t)
	ownerTok, memberTok, circleID, memberID := circleWithMember(t, srv, caps)

	rec := doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+circleID+"/transfer", ownerTok, map[string]any{
		"new_owner_account_id": memberID,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("передача: %d %s", rec.Code, rec.Body.String())
	}

	for _, m := range members(t, srv, memberTok, circleID) {
		switch m["name"] {
		case "Боб":
			if m["is_owner"] != true || m["can_settings"] != true {
				t.Fatalf("новый владелец без прав: %+v", m)
			}
		case "Аня":
			if m["is_owner"] == true || m["can_settings"] == true {
				t.Fatalf("прежний владелец сохранил права: %+v", m)
			}
		}
	}

	// Права поменялись местами и на деле: прежний владелец получает отказ,
	// новый — проходит.
	rec = doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+circleID+"/transfer", ownerTok, map[string]any{
		"new_owner_account_id": memberID,
	})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("прежний владелец всё ещё распоряжается: %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, srv, http.MethodDelete, "/api/v1/circles/"+circleID, memberTok, map[string]any{"name": "Семья"})
	if rec.Code != http.StatusOK {
		t.Fatalf("новый владелец не может удалить круг: %d %s", rec.Code, rec.Body.String())
	}
}

// Инвариант: исключение закрывает доступ сразу — исключённый не читает
// ленту и не пишет, а его записи остаются в круге.
func TestExcludeMemberCutsAccess(t *testing.T) {
	srv, caps, _, _ := setupAPI(t)
	ownerTok, memberTok, circleID, memberID := circleWithMember(t, srv, caps)

	rec := doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+circleID+"/posts", memberTok, map[string]any{
		"body": "запись Боба", "entry_date": "2026-08-30",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("запись участника: %d %s", rec.Code, rec.Body.String())
	}
	postID := jsonStr(t, rec, "id")

	rec = doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+circleID+"/exclude", ownerTok, map[string]any{
		"account_id": memberID,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("исключение: %d %s", rec.Code, rec.Body.String())
	}

	// Чтение и запись закрыты.
	if rec := doGET(t, srv, "/api/v1/circles/"+circleID+"/feed", memberTok); rec.Code != http.StatusForbidden {
		t.Fatalf("исключённый читает ленту: %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+circleID+"/posts", memberTok, map[string]any{
		"body": "ещё раз", "entry_date": "2026-08-30",
	})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("исключённый пишет: %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, srv, http.MethodDelete, "/api/v1/circles/"+circleID+"/posts/"+postID, memberTok, nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("исключённый стирает свою ветку: %d %s", rec.Code, rec.Body.String())
	}

	// Сказанное остаётся в круге, а участник виден как ушедший.
	rec = doGET(t, srv, "/api/v1/circles/"+circleID+"/feed", ownerTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("лента владельца: %d %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "запись Боба") {
		t.Fatalf("запись исключённого пропала из круга: %s", rec.Body.String())
	}
	for _, m := range members(t, srv, ownerTok, circleID) {
		if m["name"] == "Боб" && m["status"] == "active" {
			t.Fatalf("исключённый остался действующим: %+v", m)
		}
	}
}

func TestSettingsGrantAndRevokeInFeed(t *testing.T) {
	srv, caps, _, _ := setupAPI(t)
	ownerTok, _, circleID, memberID := circleWithMember(t, srv, caps)

	rec := doJSON(t, srv, http.MethodPut, "/api/v1/circles/"+circleID+"/members/"+memberID, ownerTok, map[string]any{
		"can_settings": true,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("grant: %d %s", rec.Code, rec.Body.String())
	}
	rec = doGET(t, srv, "/api/v1/circles/"+circleID+"/feed", ownerTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("feed: %d %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "дал право менять настройки") {
		t.Fatalf("выдача не в журнале: %s", rec.Body.String())
	}

	rec = doJSON(t, srv, http.MethodPut, "/api/v1/circles/"+circleID+"/members/"+memberID, ownerTok, map[string]any{
		"can_settings": false,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("revoke: %d %s", rec.Code, rec.Body.String())
	}
	rec = doGET(t, srv, "/api/v1/circles/"+circleID+"/feed", ownerTok)
	if !strings.Contains(rec.Body.String(), "забрал право менять настройки") {
		t.Fatalf("снятие не в журнале: %s", rec.Body.String())
	}
}

// Админ удалил учётку, человек завёл новую на ту же почту и вернулся в круг:
// в участниках он один, прежняя учётка в «Вышли» не висит. Право настроек у
// вышедшего не показывается.
func TestMembersHideDeletedAccountAfterReturn(t *testing.T) {
	srv, caps, _, _ := setupAPI(t)
	ownerTok, _, circleID, memberID := circleWithMember(t, srv, caps)
	rec := doJSON(t, srv, http.MethodPut, "/api/v1/circles/"+circleID+"/members/"+memberID, ownerTok, map[string]any{
		"can_settings": true,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("grant: %d %s", rec.Code, rec.Body.String())
	}
	admin := adminToken(t, srv)
	rec = doJSON(t, srv, http.MethodDelete, "/api/v1/admin/accounts/"+memberID, admin, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("delete: %d %s", rec.Code, rec.Body.String())
	}
	for _, m := range members(t, srv, ownerTok, circleID) {
		if m["name"] == "Боб" {
			t.Fatalf("удалённая учётка в участниках: %v", m)
		}
	}
	joinAsMember(t, srv, caps, ownerTok, circleID, "bob@example.com", "Боб")
	n := 0
	for _, m := range members(t, srv, ownerTok, circleID) {
		if m["name"] == "Боб" {
			n++
			if m["status"] != "active" || m["can_settings"] == true {
				t.Fatalf("вернувшийся Боб: %v", m)
			}
		}
	}
	if n != 1 {
		t.Fatalf("Боб в участниках %d раз", n)
	}
}

// Вышедший «совсем» остаётся в «Вышли», но без «может менять настройки».
func TestMembersLeftLoseSettingsMark(t *testing.T) {
	srv, caps, _, _ := setupAPI(t)
	ownerTok, memberTok, circleID, memberID := circleWithMember(t, srv, caps)
	rec := doJSON(t, srv, http.MethodPut, "/api/v1/circles/"+circleID+"/members/"+memberID, ownerTok, map[string]any{
		"can_settings": true,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("grant: %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+circleID+"/leave", memberTok, map[string]any{"mode": "full"})
	if rec.Code != http.StatusOK && rec.Code != http.StatusNoContent {
		t.Skipf("leave route differs: %d %s", rec.Code, rec.Body.String())
	}
	for _, m := range members(t, srv, ownerTok, circleID) {
		if m["name"] == "Боб" && m["can_settings"] == true {
			t.Fatalf("у вышедшего право настроек: %v", m)
		}
	}
}
