package chronicle_test

import (
	"encoding/json"
	"slices"
	"testing"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
)

// archiveOverJSON — что пришлёт устройство: строки таблиц, прошедшие через
// JSON (числа — float64), почта по учёткам и блобы, уже лежащие здесь.
func archiveOverJSON(t *testing.T, data *chronicle.FullArchiveData) (map[string]chronicle.FullArchiveTable, map[string]string, map[string]string) {
	t.Helper()
	tables := map[string]chronicle.FullArchiveTable{}
	accounts := map[string]string{}
	blobs := map[string]string{}
	for _, tb := range data.Tables {
		raw, err := json.Marshal(map[string]any{"columns": tb.Columns, "rows": tb.Rows})
		if err != nil {
			t.Fatal(err)
		}
		var back struct {
			Columns []string `json:"columns"`
			Rows    [][]any  `json:"rows"`
		}
		if err := json.Unmarshal(raw, &back); err != nil {
			t.Fatal(err)
		}
		tables[tb.Name] = chronicle.FullArchiveTable{Name: tb.Name, Columns: back.Columns, Rows: back.Rows}
		switch tb.Name {
		case "accounts":
			id, email := slices.Index(back.Columns, "id"), slices.Index(back.Columns, "email")
			for _, r := range back.Rows {
				accounts[r[id].(string)] = r[email].(string)
			}
		case "blobs":
			id := slices.Index(back.Columns, "id")
			for _, r := range back.Rows {
				blobs[r[id].(string)] = r[id].(string)
			}
		}
	}
	return tables, accounts, blobs
}

// Инвариант (план 52): круг оживает рядом с исходным — новые id, те же
// записи, комментарии, реакции и имена; исходный круг не тронут; журнал
// продолжается служебной строкой; люди находятся по почте.
func TestReviveCircleNextToOriginal(t *testing.T) {
	e := newTestEnv(t)
	e.seedAccount("owner")
	e.seedAccount("bob")
	circle := e.createCircle("owner", "Аня", chronicle.UnlimitedWindow())
	e.join(circle.ID, "bob", "Боб", e.at(0))
	p := e.post(circle.ID, "owner", "на даче", "2026-08-01", e.at(1))
	e.seedBlob("photo1", "owner")
	e.attachPhoto(p.ID, "photo1")
	if _, err := e.ch.CreateComment(e.ctx, chronicle.CommentInput{CircleID: circle.ID, AccountID: "bob", PostID: p.ID, Body: "красота", Now: e.at(2)}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.ch.SetReaction(e.ctx, chronicle.ReactionInput{CircleID: circle.ID, AccountID: "bob", PostID: p.ID, Emoji: "heart", Now: e.at(2)}); err != nil {
		t.Fatal(err)
	}
	data, err := e.ch.FullArchiveData(e.ctx, circle.ID)
	if err != nil {
		t.Fatal(err)
	}
	tables, accounts, blobs := archiveOverJSON(t, data)

	res, err := e.ch.ReviveCircle(e.ctx, chronicle.ReviveInput{
		FormatVersion: 2, SchemaVersion: 1, Tables: tables, Accounts: accounts, BlobMap: blobs,
		ReviverID: "owner", Now: e.at(3),
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.CircleID == "" || res.CircleID == circle.ID || res.Posts != 1 || res.Names != 2 || len(res.NotJoined) != 0 {
		t.Fatalf("итог: %+v", res)
	}
	feed, err := e.ch.FullArchiveFeed(e.ctx, res.CircleID, e.at(4))
	if err != nil {
		t.Fatal(err)
	}
	if len(feed) != 1 || feed[0].Post.ID == p.ID || feed[0].Post.Body != "на даче" ||
		len(feed[0].Comments) != 1 || feed[0].Comments[0].Body != "красота" ||
		len(feed[0].Reactions) != 1 || len(feed[0].Media) != 1 || feed[0].Media[0].BlobID != "photo1" {
		t.Fatalf("лента оживлённого круга: %+v", feed)
	}
	// Боб — своим именем и своей учёткой.
	if feed[0].Comments[0].AuthorName != "Боб" {
		t.Fatalf("комментарий: %+v", feed[0].Comments[0])
	}
	if ok, err := e.ch.CanWrite(e.ctx, res.CircleID, "bob", e.at(4)); err != nil || !ok {
		t.Fatalf("Боб в оживлённом круге: %v %v", ok, err)
	}
	owner, err := e.ch.CircleOwnerAccount(e.ctx, res.CircleID)
	if err != nil || owner != "owner" {
		t.Fatalf("владелец: %q %v", owner, err)
	}
	// Исходный круг на месте.
	orig, err := e.ch.FullArchiveFeed(e.ctx, circle.ID, e.at(4))
	if err != nil || len(orig) != 1 || orig[0].Post.ID != p.ID {
		t.Fatalf("исходный круг: %+v %v", orig, err)
	}
	var revived int
	if err := e.ch.DB().QueryRowContext(e.ctx, `SELECT COUNT(*) FROM events WHERE circle_id = ? AND event_type = 'circle_revived'`, res.CircleID).Scan(&revived); err != nil || revived != 1 {
		t.Fatalf("служебная строка оживления: %d %v", revived, err)
	}
}

// Тот, кто оживил, не из архива — он владелец и получает лицо прежнего
// владельца; почта, которой на сервере нет, — учётка без входа (6.6).
func TestReviveCircleByStranger(t *testing.T) {
	e := newTestEnv(t)
	e.seedAccount("owner")
	e.seedAccount("bob")
	circle := e.createCircle("owner", "Аня", chronicle.UnlimitedWindow())
	e.join(circle.ID, "bob", "Боб", e.at(1))
	e.post(circle.ID, "owner", "на даче", "2026-08-01", e.at(1))
	data, err := e.ch.FullArchiveData(e.ctx, circle.ID)
	if err != nil {
		t.Fatal(err)
	}
	tables, accounts, blobs := archiveOverJSON(t, data)
	// На «другом сервере» Боба нет: его почта незнакома.
	for id, email := range accounts {
		if email == "bob@test.local" {
			accounts[id] = "bob@elsewhere.example"
		}
	}
	e.seedAccount("slava")
	res, err := e.ch.ReviveCircle(e.ctx, chronicle.ReviveInput{
		FormatVersion: 1, SchemaVersion: 36, Tables: tables, Accounts: accounts, BlobMap: blobs, ReviverID: "slava", Now: e.at(3),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.NotJoined) != 1 {
		t.Fatalf("учётки без входа: %v", res.NotJoined)
	}
	owner, _ := e.ch.CircleOwnerAccount(e.ctx, res.CircleID)
	if owner != "slava" {
		t.Fatalf("владелец оживлённого круга: %q", owner)
	}
	feed, _ := e.ch.FullArchiveFeed(e.ctx, res.CircleID, e.at(4))
	if len(feed) != 1 || feed[0].Post.AuthorName != "Аня" {
		t.Fatalf("запись прежнего владельца: %+v", feed)
	}
	if ok, _ := e.ch.CanWrite(e.ctx, res.CircleID, "slava", e.at(4)); !ok {
		t.Fatal("ожививший не пишет в круг")
	}
}

// «Аня — это вы?» → «Нет» (A12, 7.29в): оживляет не прежний владелец —
// он входит своим именем новым участником и владельцем, видит круг с
// оживления; лицо и прошлое Ани остаются за её почтой, настройки она больше
// не меняет — как после передачи владения.
func TestReviveCircleByStrangerAsNewMember(t *testing.T) {
	e := newTestEnv(t)
	e.seedAccount("owner")
	circle := e.createCircle("owner", "Аня", chronicle.UnlimitedWindow())
	e.post(circle.ID, "owner", "на даче", "2026-08-01", e.at(1))
	data, err := e.ch.FullArchiveData(e.ctx, circle.ID)
	if err != nil {
		t.Fatal(err)
	}
	tables, accounts, blobs := archiveOverJSON(t, data)
	e.seedAccount("slava")
	res, err := e.ch.ReviveCircle(e.ctx, chronicle.ReviveInput{
		FormatVersion: 2, SchemaVersion: 1, Tables: tables, Accounts: accounts, BlobMap: blobs,
		ReviverID: "slava", Now: e.at(3),
		NewMember: &chronicle.ReviveNewMember{Name: "Слава", Gender: chronicle.GenderMale},
	})
	if err != nil {
		t.Fatal(err)
	}
	if owner, _ := e.ch.CircleOwnerAccount(e.ctx, res.CircleID); owner != "slava" {
		t.Fatalf("владелец: %q", owner)
	}
	if res.AlreadyHere != 1 {
		t.Fatalf("Аня уже на сервере: %+v", res)
	}
	type member struct {
		name     string
		settings int
		started  string
	}
	read := func(account string) member {
		var m member
		if err := e.ch.DB().QueryRowContext(e.ctx, `
			SELECT n.name, m.can_settings, MIN(s.started_at)
			FROM memberships m
			JOIN identity_names n ON n.identity_id = m.identity_id
			JOIN membership_spans s ON s.membership_id = m.id
			WHERE m.circle_id = ? AND m.account_id = ?`, res.CircleID, account).Scan(&m.name, &m.settings, &m.started); err != nil {
			t.Fatalf("участие %s: %v", account, err)
		}
		return m
	}
	slava, anya := read("slava"), read("owner")
	started, _ := time.Parse(time.RFC3339Nano, slava.started)
	if slava.name != "Слава" || slava.settings != 1 || !started.Equal(e.at(3)) {
		t.Fatalf("Слава: %+v (ждали отрезок с %s)", slava, e.at(3))
	}
	if anya.name != "Аня" || anya.settings != 0 {
		t.Fatalf("Аня: %+v", anya)
	}
	feed, _ := e.ch.FullArchiveFeed(e.ctx, res.CircleID, e.at(4))
	if len(feed) != 1 || feed[0].Post.AuthorName != "Аня" {
		t.Fatalf("запись Ани: %+v", feed)
	}
	// В журнале одна строка — «оживил круг» от Славы, без «вступил».
	var revivedBy string
	var joined int
	if err := e.ch.DB().QueryRowContext(e.ctx, `SELECT actor_name FROM events WHERE circle_id = ? AND event_type = 'circle_revived'`, res.CircleID).Scan(&revivedBy); err != nil || revivedBy != "Слава" {
		t.Fatalf("строка оживления: %q %v", revivedBy, err)
	}
	if err := e.ch.DB().QueryRowContext(e.ctx, `SELECT COUNT(*) FROM events WHERE circle_id = ? AND event_type = 'member.joined' AND created_at >= ?`, res.CircleID, "2026-08-04").Scan(&joined); err != nil || joined != 0 {
		t.Fatalf("лишняя строка «вступил»: %d %v", joined, err)
	}
}

// Архив неизвестной схемы и файл, который не загрузили, — отказ, а не
// полкруга.
func TestReviveCircleRefusesBrokenInput(t *testing.T) {
	e := newTestEnv(t)
	e.seedAccount("owner")
	circle := e.createCircle("owner", "Аня", chronicle.UnlimitedWindow())
	p := e.post(circle.ID, "owner", "на даче", "2026-08-01", e.at(0))
	e.seedBlob("photo1", "owner")
	e.attachPhoto(p.ID, "photo1")
	data, _ := e.ch.FullArchiveData(e.ctx, circle.ID)
	tables, accounts, _ := archiveOverJSON(t, data)
	if _, err := e.ch.ReviveCircle(e.ctx, chronicle.ReviveInput{FormatVersion: 2, SchemaVersion: 99, Tables: tables, Accounts: accounts, ReviverID: "owner"}); err == nil {
		t.Fatal("оживлён архив неизвестной схемы")
	}
	// Номер схемы прежней нумерации в разметке 2 и новой в разметке 1 — чужие.
	if _, err := e.ch.ReviveCircle(e.ctx, chronicle.ReviveInput{FormatVersion: 2, SchemaVersion: 36, Tables: tables, Accounts: accounts, ReviverID: "owner"}); err == nil {
		t.Fatal("оживлён архив разметки 2 со схемой 36")
	}
	if _, err := e.ch.ReviveCircle(e.ctx, chronicle.ReviveInput{FormatVersion: 1, SchemaVersion: 1, Tables: tables, Accounts: accounts, ReviverID: "owner"}); err == nil {
		t.Fatal("оживлён архив разметки 1 со схемой 1")
	}
	if _, err := e.ch.ReviveCircle(e.ctx, chronicle.ReviveInput{FormatVersion: 1, SchemaVersion: 34, Tables: tables, Accounts: accounts, BlobMap: map[string]string{}, ReviverID: "owner"}); err == nil {
		t.Fatal("оживлён круг без загруженного файла")
	}
	var n int
	_ = e.ch.DB().QueryRowContext(e.ctx, `SELECT COUNT(*) FROM circles`).Scan(&n)
	if n != 1 {
		t.Fatalf("после отказа кругов %d", n)
	}
}
