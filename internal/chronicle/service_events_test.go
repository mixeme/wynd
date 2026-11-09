package chronicle_test

import (
	"testing"

	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
)

// C82: смена цвета и настроек приглашений — изменения структуры круга,
// поэтому каждая оставляет служебную строку журнала.
func TestCircleSettingsLeaveServiceLines(t *testing.T) {
	e := newTestEnv(t)
	circle := e.createCircle("owner", "Аня", chronicle.UnlimitedWindow())

	if err := e.ch.SetCircleColor(e.ctx, circle.ID, "owner", "olive", e.at(0)); err != nil {
		t.Fatalf("SetCircleColor: %v", err)
	}
	if err := e.ch.SetInviteWho(e.ctx, circle.ID, "owner", "owner", e.at(0)); err != nil {
		t.Fatalf("SetInviteWho: %v", err)
	}
	if err := e.ch.SetInviteKindDefault(e.ctx, circle.ID, "owner", "multi", e.at(0)); err != nil {
		t.Fatalf("SetInviteKindDefault: %v", err)
	}
	// Повтор тех же значений — без новых строк: журнал не шумит.
	if err := e.ch.SetCircleColor(e.ctx, circle.ID, "owner", "olive", e.at(0)); err != nil {
		t.Fatalf("SetCircleColor repeat: %v", err)
	}
	if err := e.ch.SetInviteWho(e.ctx, circle.ID, "owner", "owner", e.at(0)); err != nil {
		t.Fatalf("SetInviteWho repeat: %v", err)
	}
	if err := e.ch.SetInviteKindDefault(e.ctx, circle.ID, "owner", "multi", e.at(0)); err != nil {
		t.Fatalf("SetInviteKindDefault repeat: %v", err)
	}

	rows, err := e.ch.DB().QueryContext(e.ctx, `
		SELECT event_type, summary FROM events
		WHERE circle_id = ? AND event_type IN ('circle.color_changed', 'circle.invite_settings_changed')
		ORDER BY seq
	`, circle.ID)
	if err != nil {
		t.Fatalf("read events: %v", err)
	}
	defer rows.Close()

	want := []struct{ typ, summary string }{
		{"circle.color_changed", "Цвет круга изменён"},
		{"circle.invite_settings_changed", "Создавать приглашения: только владелец"},
		{"circle.invite_settings_changed", "Приглашения: многоразовые"},
	}
	i := 0
	for rows.Next() {
		var typ, summary string
		if err := rows.Scan(&typ, &summary); err != nil {
			t.Fatalf("scan: %v", err)
		}
		if i >= len(want) {
			t.Fatalf("лишняя строка: %s %q", typ, summary)
		}
		if typ != want[i].typ || summary != want[i].summary {
			t.Fatalf("строка %d: got %s %q, want %s %q", i, typ, summary, want[i].typ, want[i].summary)
		}
		i++
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows: %v", err)
	}
	if i != len(want) {
		t.Fatalf("строк структуры: %d, want %d", i, len(want))
	}
}

// C82: по «Отсечке» и «Сроку» конец цикла не отличить от идущего, поэтому
// удаление после отсечки пишет итог со счётом.
func TestArchivePurgeLeavesSummaryLine(t *testing.T) {
	e := newTestEnv(t)
	circle := e.createCircle("owner", "Аня", chronicle.UnlimitedWindow())
	e.post(circle.ID, "owner", "удалится", "2026-08-01", e.at(0))
	if err := e.ch.StartArchiveCycle(e.ctx, circle.ID, "owner", "2026-08-10", e.at(5), 86400, e.at(1)); err != nil {
		t.Fatal(err)
	}
	if _, err := e.ch.PurgeBeforeCutoff(e.ctx, circle.ID, "2026-08-10", e.at(6)); err != nil {
		t.Fatal(err)
	}

	var summary string
	err := e.ch.DB().QueryRowContext(e.ctx, `
		SELECT summary FROM events WHERE circle_id = ? AND event_type = 'archive_purged'
	`, circle.ID).Scan(&summary)
	if err != nil {
		t.Fatalf("итог архивации не записан: %v", err)
	}
	if want := "Архивация завершена: удалено 1 запись"; summary != want {
		t.Fatalf("итог архивации: %q, want %q", summary, want)
	}
}

// Склонение счёта: «2 записи», «3 файла».
func TestArchivePurgeSummaryPlurals(t *testing.T) {
	e := newTestEnv(t)
	circle := e.createCircle("owner", "Аня", chronicle.UnlimitedWindow())
	p1 := e.post(circle.ID, "owner", "раз", "2026-08-01", e.at(0))
	p2 := e.post(circle.ID, "owner", "два", "2026-08-02", e.at(0))
	e.seedBlob("b1", "owner")
	e.seedBlob("b2", "owner")
	e.seedBlob("b3", "owner")
	e.attachPhoto(p1.ID, "b1")
	e.attachPhoto(p1.ID, "b2")
	e.attachPhoto(p2.ID, "b3")
	if err := e.ch.StartArchiveCycle(e.ctx, circle.ID, "owner", "2026-08-10", e.at(5), 86400, e.at(1)); err != nil {
		t.Fatal(err)
	}
	if _, err := e.ch.PurgeBeforeCutoff(e.ctx, circle.ID, "2026-08-10", e.at(6)); err != nil {
		t.Fatal(err)
	}
	var summary string
	if err := e.ch.DB().QueryRowContext(e.ctx, `
		SELECT summary FROM events WHERE circle_id = ? AND event_type = 'archive_purged'
	`, circle.ID).Scan(&summary); err != nil {
		t.Fatal(err)
	}
	if want := "Архивация завершена: удалено 2 записи, 3 файла"; summary != want {
		t.Fatalf("итог архивации: %q, want %q", summary, want)
	}
}
