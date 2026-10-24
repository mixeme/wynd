package chronicle_test

import (
	"errors"
	"testing"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
)

// «Отделиться» (A9): в новый круг уходит то, что видел отделяющийся, —
// записи и комментарии всех авторов; позванный входит сам и видит прошлое
// только там, где видел его в старом круге; тот, от кого отделились, в
// новый круг не попадает; старый круг не меняется; файлы общие.
func TestSplitCircle(t *testing.T) {
	e := newTestEnv(t)
	for _, id := range []string{"cat", "bob", "mouse", "anya"} {
		e.seedAccount(id)
	}
	circle := e.createCircle("cat", "Кот", chronicle.UnlimitedWindow())
	e.join(circle.ID, "bob", "Боб", e.after(time.Hour))
	e.post(circle.ID, "cat", "до Мыши", "2026-08-01", e.after(2*time.Hour))
	e.join(circle.ID, "mouse", "Мышь", e.at(1))
	e.post(circle.ID, "bob", "Боб до Ани", "2026-08-03", e.at(2))
	e.join(circle.ID, "anya", "Аня", e.at(3))
	p := e.post(circle.ID, "anya", "на даче", "2026-08-05", e.at(4))
	e.seedBlob("photo1", "anya")
	e.attachPhoto(p.ID, "photo1")
	if _, err := e.ch.CreateComment(e.ctx, chronicle.CommentInput{CircleID: circle.ID, AccountID: "bob", PostID: p.ID, Body: "красота", Now: e.at(4).Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	var oldEvents int
	if err := e.ch.DB().QueryRowContext(e.ctx, `SELECT COUNT(*) FROM events WHERE circle_id = ?`, circle.ID).Scan(&oldEvents); err != nil {
		t.Fatal(err)
	}

	pre, err := e.ch.SplitPreview(e.ctx, circle.ID, "mouse", e.at(5))
	if err != nil {
		t.Fatal(err)
	}
	if pre.Posts != 2 || pre.Since == nil || !pre.Since.Equal(e.at(1)) || len(pre.People) != 3 {
		t.Fatalf("предпросмотр: %+v", pre)
	}
	if _, err := e.ch.SplitCircle(e.ctx, chronicle.SplitInput{CircleID: circle.ID, AccountID: "mouse", Name: " ", Now: e.at(5)}); !errors.Is(err, chronicle.ErrInvalid) {
		t.Fatalf("без имени: %v", err)
	}
	if _, err := e.ch.SplitCircle(e.ctx, chronicle.SplitInput{CircleID: circle.ID, AccountID: "mouse", Name: "Наша семья", Invite: []string{"stranger"}, Now: e.at(5)}); !errors.Is(err, chronicle.ErrInvalid) {
		t.Fatalf("позвать чужого: %v", err)
	}
	res, err := e.ch.SplitCircle(e.ctx, chronicle.SplitInput{
		CircleID: circle.ID, AccountID: "mouse", Name: "Наша семья", Invite: []string{"anya"}, Now: e.at(5),
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.CircleID == "" || res.CircleID == circle.ID || res.Posts != 2 || len(res.Invited) != 1 {
		t.Fatalf("итог: %+v", res)
	}

	// Тот, кто отделился, — владелец и видит всё перешедшее.
	feed, err := e.ch.FeedSnapshot(e.ctx, res.CircleID, "mouse")
	if err != nil {
		t.Fatal(err)
	}
	bodies := map[string]int{}
	for _, f := range feed {
		bodies[f.Post.Body] = len(f.Comments)
		for _, m := range f.Media {
			if m.BlobID != "photo1" {
				t.Fatalf("файл не общий: %+v", m)
			}
		}
	}
	if len(feed) != 2 || bodies["Боб до Ани"] != 0 || bodies["на даче"] != 1 {
		t.Fatalf("лента отделившегося: %v", bodies)
	}
	// Перенесённое — не новое: его видели в старом круге.
	if n, err := e.ch.UnreadResponses(e.ctx, res.CircleID, "mouse"); err != nil || n != 0 {
		t.Fatalf("отклики отделившейся: %d %v", n, err)
	}
	if ok, err := e.ch.CanWrite(e.ctx, res.CircleID, "mouse", e.at(6)); err != nil || !ok {
		t.Fatalf("писать в новом круге: %v %v", ok, err)
	}
	name, err := e.ch.CircleName(e.ctx, res.CircleID)
	if err != nil || name != "Наша семья" {
		t.Fatalf("имя: %q %v", name, err)
	}

	// Тот, от кого отделились, в новом круге не участник.
	if _, err := e.ch.MembershipForAccount(e.ctx, res.CircleID, "bob"); !errors.Is(err, chronicle.ErrNotFound) {
		t.Fatalf("Боб в новом круге: %v", err)
	}
	// Позванная до входа круга не видит, после — только своё прошлое.
	if _, err := e.ch.FeedSnapshot(e.ctx, res.CircleID, "anya"); err == nil {
		t.Fatal("позванная видит круг до входа")
	}
	info, err := e.ch.SplitJoinFor(e.ctx, res.CircleID, "anya", "mouse")
	if err != nil || info == nil || info.From != "Семья" || info.YouName != "Аня" || info.PastFrom == nil || !info.PastFrom.Equal(e.at(3)) ||
		info.InviteLine != "Круг отделён от круга «Семья», Мышь зовёт вас" {
		t.Fatalf("экран «позвали»: %+v %v", info, err)
	}
	e.join(res.CircleID, "anya", "Аня", e.at(6))
	feed, err = e.ch.FeedSnapshot(e.ctx, res.CircleID, "anya")
	if err != nil {
		t.Fatal(err)
	}
	// Комментарий Боба к записи Ани был и в старом круге — в «Откликах» не новый.
	if n, err := e.ch.UnreadResponses(e.ctx, res.CircleID, "anya"); err != nil || n != 0 {
		t.Fatalf("отклики позванной: %d %v", n, err)
	}
	if len(feed) != 1 || feed[0].Post.Body != "на даче" {
		t.Fatalf("лента позванной: %+v", feed)
	}
	var names int
	if err := e.ch.DB().QueryRowContext(e.ctx, `
		SELECT COUNT(*) FROM identity_names n JOIN identities i ON i.id = n.identity_id
		WHERE i.circle_id = ? AND i.account_id = 'anya'`, res.CircleID).Scan(&names); err != nil || names != 1 {
		t.Fatalf("имя Ани записано заново: %d %v", names, err)
	}

	// Граница в ленте и тишина в старом круге.
	var split string
	if err := e.ch.DB().QueryRowContext(e.ctx, `SELECT summary FROM events WHERE circle_id = ? AND event_type = ?`,
		res.CircleID, chronicle.EventCircleSplit).Scan(&split); err != nil || split != "Круг отделён от круга «Семья»" {
		t.Fatalf("служебная строка: %q %v", split, err)
	}
	var after int
	if err := e.ch.DB().QueryRowContext(e.ctx, `SELECT COUNT(*) FROM events WHERE circle_id = ?`, circle.ID).Scan(&after); err != nil || after != oldEvents {
		t.Fatalf("в старом круге строк: было %d, стало %d (%v)", oldEvents, after, err)
	}
	if info, err := e.ch.SplitJoinFor(e.ctx, circle.ID, "anya", ""); err != nil || info != nil {
		t.Fatalf("старый круг не отделён: %+v %v", info, err)
	}
}

// Отделиться может только тот, кто в круге.
func TestSplitCircleNeedsActiveMember(t *testing.T) {
	e := newTestEnv(t)
	e.seedAccount("cat")
	e.seedAccount("mouse")
	circle := e.createCircle("cat", "Кот", chronicle.UnlimitedWindow())
	e.join(circle.ID, "mouse", "Мышь", e.at(1))
	if err := e.ch.Leave(e.ctx, circle.ID, "mouse", e.at(2)); err != nil {
		t.Fatal(err)
	}
	if _, err := e.ch.SplitCircle(e.ctx, chronicle.SplitInput{CircleID: circle.ID, AccountID: "mouse", Name: "Своё", Now: e.at(3)}); !errors.Is(err, chronicle.ErrForbidden) {
		t.Fatalf("вышедшая отделяется: %v", err)
	}
}
