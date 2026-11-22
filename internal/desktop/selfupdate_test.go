package desktop

import (
	"testing"
	"time"
)

// Окно предлагает свежий выпуск, вышедший не меньше UpdateDelay назад;
// черновики, пробные и чужие метки — нет. Ведёт себя как Мастерская.
func TestPickUpdate(t *testing.T) {
	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	old := now.Add(-10 * 24 * time.Hour)
	assets := []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
	}{{"wynd-desktop.exe", "https://x/exe"}, {"wynd-desktop.exe.sig", "https://x/sig"}}
	rels := []ghRelease{
		{TagName: "v0.49.0", PublishedAt: old},
		{TagName: "desktop-v0.2.0", PublishedAt: now.Add(-time.Hour), Assets: assets},
		{TagName: "desktop-v0.1.9", PublishedAt: old, Draft: true, Assets: assets},
		{TagName: "desktop-v0.1.5", PublishedAt: old, Assets: assets, Body: "- Список серверов\n- Самообновление\n"},
	}
	up := pickUpdate(rels, "0.1.1", now)
	if !up.Available || up.Version != "0.1.5" || len(up.Notes) != 2 {
		t.Fatalf("выпуск: %+v", up)
	}
	if up := pickUpdate(rels, "0.1.5", now); up.Available {
		t.Fatalf("новее нет, а предложено: %+v", up)
	}
	// Без подписи в выпуске установка не предлагается.
	noSig := []ghRelease{{TagName: "desktop-v0.2.0", PublishedAt: old, Assets: assets[:1]}}
	if up := pickUpdate(noSig, "0.1.1", now); up.Available {
		t.Fatalf("выпуск без подписи предложен: %+v", up)
	}
}

// Уведомление о новой версии: один раз на версию, без обновления — молча.
func TestNotifyUpdateText(t *testing.T) {
	up := Update{Available: true, Version: "0.4.3"}
	title, body, ok := NotifyUpdateText(up, "")
	if !ok || title != "Вышла версия 0.4.3" || body == "" {
		t.Fatalf("первое уведомление: %q, %q, %v", title, body, ok)
	}
	if _, _, ok := NotifyUpdateText(up, "0.4.3"); ok {
		t.Error("уведомление той же версии повторено")
	}
	if _, _, ok := NotifyUpdateText(Update{}, ""); ok {
		t.Error("уведомление без обновления")
	}
}
