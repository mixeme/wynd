package chronicle

import (
	"testing"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/store"
)

// Инвариант (план 46, C7): шаг столбиков — по возрасту круга; неделя
// начинается с понедельника, столбик несёт дату своего начала.
func TestVolumeStepAndPeriods(t *testing.T) {
	day := 24 * time.Hour
	for _, c := range []struct {
		age  time.Duration
		want VolumeStep
	}{
		{6 * day, VolumeStepDay},
		{14 * day, VolumeStepDay},
		{15 * day, VolumeStepWeek},
		{92 * day, VolumeStepWeek},
		{93 * day, VolumeStepMonth},
	} {
		if got := volumeStepFor(c.age); got != c.want {
			t.Errorf("age %v: %s, want %s", c.age, got, c.want)
		}
	}

	st, err := store.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	c, err := New(st)
	if err != nil {
		t.Fatal(err)
	}
	// 2026-10-01 — четверг; понедельник той недели — 2026-09-28.
	for step, want := range map[VolumeStep]string{
		VolumeStepDay:   "2026-10-01",
		VolumeStepWeek:  "2026-09-28",
		VolumeStepMonth: "2026-10-01",
	} {
		var got string
		q := `SELECT ` + volumePeriodSQL[step] + ` FROM (SELECT '2026-10-01T15:04:05Z' AS created_at) p`
		if err := c.db.QueryRow(q).Scan(&got); err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Errorf("%s period: %s, want %s", step, got, want)
		}
	}
	var monday string
	if err := c.db.QueryRow(`SELECT ` + volumePeriodSQL[VolumeStepWeek] + ` FROM (SELECT '2026-09-28T01:00:00Z' AS created_at) p`).Scan(&monday); err != nil {
		t.Fatal(err)
	}
	if monday != "2026-09-28" {
		t.Errorf("monday stays monday: %s", monday)
	}
}
