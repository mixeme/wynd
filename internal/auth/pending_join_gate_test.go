package auth_test

import (
	"testing"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/auth"
	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
)

// circleWithInvite заводит учётку-владельца, её круг и ссылку в него.
func circleWithInvite(t *testing.T, e *env) (ownerID, circleID string, inv auth.Invite) {
	t.Helper()
	if err := e.auth.SetRegistrationMode(e.ctx, auth.ModeOpen); err != nil {
		t.Fatal(err)
	}
	if err := e.auth.Register(e.ctx, auth.RegisterInput{
		Email: "ana@example.com", ClientIP: "10.0.0.1", Now: e.t0,
	}); err != nil {
		t.Fatal(err)
	}
	res, err := e.auth.Verify(e.ctx, auth.VerifyInput{
		Email: "ana@example.com", Code: e.caps.Last("ana@example.com"),
		ClientIP: "10.0.0.1", Now: e.t0,
	})
	if err != nil {
		t.Fatal(err)
	}
	ownerID = res.Account.ID
	circle, _, _, err := e.ch.CreateCircle(e.ctx, chronicle.CreateCircleInput{
		Name: "Семья", OwnerAccountID: ownerID, OwnerName: "Аня", Now: e.t0,
	})
	if err != nil {
		t.Fatal(err)
	}
	circleID = circle.ID
	inv, err = e.auth.CreateInvite(e.ctx, auth.CreateInviteInput{
		CircleID: circleID, Kind: auth.InviteMulti, MaxUses: 5,
		TTL: 24 * time.Hour, CreatedByAccountID: ownerID, Now: e.t0,
	})
	if err != nil {
		t.Fatal(err)
	}
	return ownerID, circleID, inv
}

// acceptWithoutName проходит ссылку без имени: имя спрашивают потом, и до
// него право войти держит строка pending_circle_joins.
func acceptWithoutName(t *testing.T, e *env, inv auth.Invite, email string, when time.Time) string {
	t.Helper()
	if err := e.auth.AcceptInvite(e.ctx, auth.AcceptInviteInput{
		Token: inv.Token, Email: email, ClientIP: "10.0.0.2", Now: when,
	}); err != nil {
		t.Fatal(err)
	}
	res, err := e.auth.Verify(e.ctx, auth.VerifyInput{
		Email: email, Code: e.caps.Last(email), ClientIP: "10.0.0.2", Now: when,
	})
	if err != nil {
		t.Fatal(err)
	}
	return res.Account.ID
}

// Инвариант (SEC-9, аудит 2026-09-22): отзыв ссылки снимает и отложенное
// вступление, заведённое по ней. Строка pending_circle_joins — это и есть
// право войти в круг, и жила она вечно.
func TestRevokedInviteDropsPendingJoin(t *testing.T) {
	e := newEnv(t)
	e.bootstrap(t)
	_, circleID, inv := circleWithInvite(t, e)
	bobID := acceptWithoutName(t, e, inv, "bob@example.com", e.t0)

	ok, err := e.auth.HasPendingCircleJoin(e.ctx, bobID, circleID, e.t0.Add(time.Minute))
	if err != nil || !ok {
		t.Fatalf("после ссылки без имени нет отложенного вступления: %v %v", ok, err)
	}
	if err := e.auth.RevokeInvite(e.ctx, inv.ID); err != nil {
		t.Fatal(err)
	}
	ok, err = e.auth.HasPendingCircleJoin(e.ctx, bobID, circleID, e.t0.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("отозванная ссылка оставила право войти")
	}
	if err := e.auth.CompleteCircleJoin(e.ctx, auth.CompleteCircleJoinInput{
		AccountID: bobID, CircleID: circleID, Name: "Боб", Now: e.t0.Add(time.Minute),
	}); err != auth.ErrForbidden {
		t.Fatalf("вступление по отозванной ссылке: %v", err)
	}
}

// Инвариант (SEC-9): право доназвать себя живёт сутки, а не вечно.
func TestPendingJoinExpires(t *testing.T) {
	e := newEnv(t)
	e.bootstrap(t)
	_, circleID, inv := circleWithInvite(t, e)
	bobID := acceptWithoutName(t, e, inv, "bob@example.com", e.t0)

	if err := e.auth.CompleteCircleJoin(e.ctx, auth.CompleteCircleJoinInput{
		AccountID: bobID, CircleID: circleID, Name: "Боб",
		Now: e.t0.Add(25 * time.Hour),
	}); err != auth.ErrForbidden {
		t.Fatalf("просроченное вступление прошло: %v", err)
	}
	// В тот же день — проходит.
	if err := e.auth.CompleteCircleJoin(e.ctx, auth.CompleteCircleJoinInput{
		AccountID: bobID, CircleID: circleID, Name: "Боб",
		Now: e.t0.Add(time.Hour),
	}); err != nil {
		t.Fatalf("вступление в срок отвергнуто: %v", err)
	}
}

// Инвариант (SEC-9): блокировка учётки отзывает выданные ею ссылки.
func TestBlockedAuthorInvitesRevoked(t *testing.T) {
	e := newEnv(t)
	e.bootstrap(t)
	ownerID, _, inv := circleWithInvite(t, e)

	if err := e.auth.SetAccountBlocked(e.ctx, ownerID, true); err != nil {
		t.Fatal(err)
	}
	if err := e.auth.AcceptInvite(e.ctx, auth.AcceptInviteInput{
		Token: inv.Token, Email: "bob@example.com", ClientIP: "10.0.0.2", Now: e.t0,
	}); err != auth.ErrInvalid {
		t.Fatalf("ссылка заблокированного всё ещё работает: %v", err)
	}
}

// Инвариант (SEC-9): ушедший или исключённый не оставляет живой ссылки в
// круг — по ней человек вошёл бы уже без пригласившего.
func TestDepartedMemberInvitesRevoked(t *testing.T) {
	e := newEnv(t)
	e.bootstrap(t)
	ownerID, circleID, inv := circleWithInvite(t, e)
	bobID := acceptWithoutName(t, e, inv, "bob@example.com", e.t0)

	if err := e.auth.RevokeCircleInvitesBy(e.ctx, circleID, ownerID, e.t0.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := e.auth.AcceptInvite(e.ctx, auth.AcceptInviteInput{
		Token: inv.Token, Email: "zoe@example.com", ClientIP: "10.0.0.3",
		Now: e.t0.Add(time.Hour),
	}); err != auth.ErrInvalid {
		t.Fatalf("ссылка ушедшего всё ещё работает: %v", err)
	}
	ok, err := e.auth.HasPendingCircleJoin(e.ctx, bobID, circleID, e.t0.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("отложенное вступление по снятой ссылке осталось")
	}
}

// Инвариант (SEC-9): у ссылки есть потолки — тысяча входов и год, одни и те
// же для админа и участника: ссылка участника не шире ссылки админа.
func TestInviteLimitsCapped(t *testing.T) {
	e := newEnv(t)
	e.bootstrap(t)
	if _, err := e.auth.CreateInvite(e.ctx, auth.CreateInviteInput{
		Kind: auth.InviteMulti, MaxUses: 1001, TTL: time.Hour, Now: e.t0,
	}); err != auth.ErrInvalid {
		t.Fatalf("тысяча один вход: %v", err)
	}
	if _, err := e.auth.CreateInvite(e.ctx, auth.CreateInviteInput{
		Kind: auth.InviteMulti, MaxUses: 5, TTL: 366 * 24 * time.Hour, Now: e.t0,
	}); err != auth.ErrInvalid {
		t.Fatalf("срок больше года: %v", err)
	}
	if _, err := e.auth.CreateInvite(e.ctx, auth.CreateInviteInput{
		Kind: auth.InviteMulti, MaxUses: 1000, TTL: 365 * 24 * time.Hour, Now: e.t0,
	}); err != nil {
		t.Fatalf("предельные значения отвергнуты: %v", err)
	}
	if _, err := e.auth.CreateServerInvite(e.ctx, auth.CreateServerInviteInput{
		Kind: auth.InviteMulti, MaxUses: 1001, TTL: time.Hour, Now: e.t0,
	}); err != auth.ErrInvalid {
		t.Fatalf("потолок входов обходится ссылкой на сервер: %v", err)
	}
}

// «Без ограничений» и «без срока» — только у ссылки админа на сервер.
// Хранятся отметками: проверки входов и срока работают как у обычной ссылки.
func TestServerInviteUnlimitedAndNoExpiry(t *testing.T) {
	e := newEnv(t)
	e.bootstrap(t)
	inv, err := e.auth.CreateServerInvite(e.ctx, auth.CreateServerInviteInput{
		Kind: auth.InviteMulti, UnlimitedUses: true, NoExpiry: true, Now: e.t0,
	})
	if err != nil {
		t.Fatalf("вечная ссылка админа: %v", err)
	}
	if inv.MaxUses != auth.UnlimitedUses || !inv.ExpiresAt.Equal(auth.NoExpiry) {
		t.Fatalf("отметки: max_uses=%d expires=%s", inv.MaxUses, inv.ExpiresAt)
	}
	// Одноразовая остаётся одноразовой, даже если попросить «без ограничений».
	single, err := e.auth.CreateServerInvite(e.ctx, auth.CreateServerInviteInput{
		Kind: auth.InviteSingle, UnlimitedUses: true, TTL: time.Hour, Now: e.t0,
	})
	if err != nil || single.MaxUses != 1 {
		t.Fatalf("одноразовая: %+v %v", single, err)
	}
}
