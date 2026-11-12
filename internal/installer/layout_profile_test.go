package installer

import (
	"path/filepath"
	"testing"
	"time"
)

// Каталог установки в профиле (план «другой каталог установки Docker»):
// сохраняется, переживает перезапуск Мастерской и передаётся мастеру сервера.
// Устоявшуюся установку правкой профиля не перенаправить в другую папку.

func TestProfileKeepsInstallRoot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "servers.json")
	vault := memVault{}
	w, err := OpenWorkshop(path, func() *Wizard { return &Wizard{Vault: vault} })
	if err != nil {
		t.Fatal(err)
	}
	p, err := w.SaveProfile(ServerProfile{Name: "Свой", Host: "srv.example.org", User: "root", UseKeys: true, Root: "/srv/wynd"})
	if err != nil {
		t.Fatalf("профиль не сохранён: %v", err)
	}
	if p.Root != "/srv/wynd" {
		t.Fatalf("каталог не сохранён: %q", p.Root)
	}
	w.Close()

	again, err := OpenWorkshop(path, func() *Wizard { return &Wizard{Vault: memVault{}} })
	if err != nil {
		t.Fatal(err)
	}
	defer again.Close()
	list := again.Profiles()
	if len(list) != 1 || list[0].Root != "/srv/wynd" {
		t.Fatalf("после перезапуска: %+v", list)
	}
	// Мастер профиля считает пути от того же каталога.
	wiz := again.makeWizard(p.ID)
	if wiz.Root != "/srv/wynd" || wiz.layout().Root() != "/srv/wynd" || wiz.layout().Data() != "/srv/wynd/data" {
		t.Fatalf("мастер не получил каталог: %q, %+v", wiz.Root, wiz.layout())
	}
}

func TestProfileRejectsBadRoot(t *testing.T) {
	w, _ := testWorkshop(t)
	defer w.Close()
	if _, err := w.SaveProfile(ServerProfile{Name: "Плохой", Host: "srv.example.org", User: "root", UseKeys: true, Root: "/etc"}); err == nil {
		t.Fatal("системный каталог принят профилем")
	}
}

// Установка уже есть, профиль хранит другой каталог: простая правка профиля
// не должна увести обновление и удаление в чужую папку.
func TestProfileKeepsRootOfExistingInstallation(t *testing.T) {
	w, _ := testWorkshop(t)
	defer w.Close()
	p, err := w.SaveProfile(ServerProfile{Name: "Свой", Host: "srv.example.org", User: "root", UseKeys: true, Root: "/srv/wynd"})
	if err != nil {
		t.Fatal(err)
	}
	if err := w.RememberCheck(p.ID, ServerCheck{At: time.Now().UTC(), Installed: true, Root: "/srv/wynd"}); err != nil {
		t.Fatal(err)
	}
	// Пустой каталог в правке — остаётся прежний.
	p.Root = ""
	edited, err := w.SaveProfile(p)
	if err != nil {
		t.Fatalf("правка без каталога: %v", err)
	}
	if edited.Root != "/srv/wynd" {
		t.Fatalf("каталог потерян: %q", edited.Root)
	}
	// Другой каталог при известной установке — отказ.
	p.Root = "/mnt/storage/wynd"
	if _, err := w.SaveProfile(p); err == nil {
		t.Fatal("смена каталога у установленного сервера принята")
	}
}
