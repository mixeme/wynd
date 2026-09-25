package jobs_test

import (
	"context"
	"errors"
	"sort"
	"testing"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
	"gitea.mixdep.ru/mix/wynd/internal/jobs"
)

// Инвариант (план 42, ARC-5): отказ письма одному адресату не прерывает
// рассылку остальным и не оставляет напоминание неотмеченным, если хоть одно
// письмо ушло; круг, где не ушло ни одного, повторяется в следующий прогон,
// а соседний круг обрабатывается независимо.
func TestArchiveReminderSurvivesOneBadAddress(t *testing.T) {
	st := openDB(t)
	ctx := t.Context()
	ch, err := chronicle.New(st)
	if err != nil {
		t.Fatal(err)
	}
	created := "2026-09-01T00:00:00.000000000Z"
	insertAccount(t, st, "owner-a", "a@example.com", created)
	insertAccount(t, st, "bad-a", "0bad@example.com", created)
	insertAccount(t, st, "owner-b", "dead@example.com", created)

	now := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)
	circleA, _, _, err := ch.CreateCircle(ctx, chronicle.CreateCircleInput{Name: "A", OwnerAccountID: "owner-a", OwnerName: "Аня", Now: now})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := ch.Join(ctx, chronicle.JoinInput{CircleID: circleA.ID, AccountID: "bad-a", Name: "Боря", Now: now}); err != nil {
		t.Fatal(err)
	}
	circleB, _, _, err := ch.CreateCircle(ctx, chronicle.CreateCircleInput{Name: "B", OwnerAccountID: "owner-b", OwnerName: "Вера", Now: now})
	if err != nil {
		t.Fatal(err)
	}
	deadline := now.Add(48 * time.Hour)
	for _, c := range []struct{ id, owner string }{{circleA.ID, "owner-a"}, {circleB.ID, "owner-b"}} {
		if err := ch.StartArchiveCycle(ctx, c.id, c.owner, "2026-09-25", deadline, int64((72 * time.Hour).Seconds()), now); err != nil {
			t.Fatal(err)
		}
	}

	var sent []string
	send := func(_ context.Context, email, _ string, _ time.Time, _ string) error {
		if email == "0bad@example.com" || email == "dead@example.com" {
			return errors.New("relay refused")
		}
		sent = append(sent, email)
		return nil
	}
	counts, err := jobs.RunArchiveJobsWithSender(ctx, st.DB(), ch, send, "https://wynd.example", now)
	if err == nil {
		t.Fatal("ожидалась ошибка отказавших адресов")
	}
	if counts.RemindersSent != 1 {
		t.Fatalf("RemindersSent = %d, want 1 (круг A)", counts.RemindersSent)
	}
	sort.Strings(sent)
	if len(sent) != 1 || sent[0] != "a@example.com" {
		t.Fatalf("sent = %v", sent)
	}

	cycleA, err := ch.GetArchiveCycle(ctx, circleA.ID)
	if err != nil {
		t.Fatal(err)
	}
	if cycleA.ReminderSentAt == nil {
		t.Fatal("круг A: письмо ушло одному из двух — напоминание должно быть отмечено")
	}
	cycleB, err := ch.GetArchiveCycle(ctx, circleB.ID)
	if err != nil {
		t.Fatal(err)
	}
	if cycleB.ReminderSentAt != nil {
		t.Fatal("круг B: не ушло ни одного письма — отметки быть не должно")
	}

	// Следующий прогон: A больше не шлёт, B пробует снова.
	sent = nil
	if _, err := jobs.RunArchiveJobsWithSender(ctx, st.DB(), ch, send, "https://wynd.example", now.Add(time.Hour)); err == nil {
		t.Fatal("ожидалась ошибка круга B")
	}
	if len(sent) != 0 {
		t.Fatalf("повторная рассылка кругу A: %v", sent)
	}
}
