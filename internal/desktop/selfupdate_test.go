package desktop

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Окно предлагает свежий выпуск сразу после публикации (до 1.0.0 отлёжка
// снята); черновики, пробные и чужие метки — нет. Ведёт себя как Мастерская.
func TestPickUpdate(t *testing.T) {
	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	old := now.Add(-10 * 24 * time.Hour)
	assets := []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
	}{{"wynd-desktop.exe", "https://x/exe"}, {"wynd-desktop.exe.sig", "https://x/sig"}}
	rels := []ghRelease{
		{TagName: "v0.49.0", PublishedAt: old},
		{TagName: "desktop-v0.2.0", PublishedAt: now.Add(-time.Hour), Assets: assets, Body: "- Список серверов\n- Самообновление\n"},
		{TagName: "desktop-v0.1.9", PublishedAt: old, Draft: true, Assets: assets},
		{TagName: "desktop-v0.1.5", PublishedAt: old, Assets: assets},
	}
	up := pickUpdate(rels, "0.1.1", now)
	if !up.Available || up.Version != "0.2.0" || len(up.Notes) != 2 {
		t.Fatalf("выпуск: %+v", up)
	}
	if up := pickUpdate(rels, "0.2.0", now); up.Available {
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

// Подмена exe самообновлением видна по прежнему образу рядом: по этому
// признаку окно ждёт выхода прежнего процесса, прежде чем брать замок одного
// экземпляра (живая проверка 27.11: новый запуск уходил в прежний и полоса
// оставалась на «Обновляем…»).
func TestFreshlyReplaced(t *testing.T) {
	exe := filepath.Join(t.TempDir(), "wynd-desktop.exe")
	if err := os.WriteFile(exe, []byte("new"), 0o755); err != nil {
		t.Fatal(err)
	}
	if FreshlyReplaced(exe) {
		t.Error("без .old exe не подменён")
	}
	if FreshlyReplaced("") {
		t.Error("пустой путь считается подменой")
	}
	if err := os.WriteFile(exe+".old", []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	if !FreshlyReplaced(exe) {
		t.Error("с .old exe подменён, а признак не найден")
	}
	if err := os.Remove(exe + ".old"); err != nil {
		t.Fatal(err)
	}
	if FreshlyReplaced(exe) {
		t.Error("после уборки .old exe снова не подменён")
	}
}
