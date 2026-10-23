package api

import "time"

// greetingDay — день первой записи при входе в круг и при оживлении (1.4,
// 7.29в): день устройства, если он рядом с сейчас, иначе — день по UTC.
// Сервер считал по UTC, и приветствие, написанное по Москве после полуночи,
// вставало на вчера (2026-10-24).
func greetingDay(client string, now time.Time) string {
	if d, err := time.Parse("2006-01-02", client); err == nil {
		if diff := d.Sub(now.UTC().Truncate(24 * time.Hour)); diff >= -36*time.Hour && diff <= 36*time.Hour {
			return client
		}
	}
	return now.UTC().Format("2006-01-02")
}
