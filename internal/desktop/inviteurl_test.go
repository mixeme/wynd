package desktop

import "testing"

func TestParseInviteURL(t *testing.T) {
	link, ok := ParseInviteURL(`wynd:open?url=https%3A%2F%2Fhome.example.org%2Finvite%2Ftok1`)
	if !ok || link != "https://home.example.org/invite/tok1" {
		t.Fatalf("закодированный адрес: %q ok=%v", link, ok)
	}
	// Шелл Windows приводит схему и «open» к нижнему регистру произвольно.
	if link, ok = ParseInviteURL(`WYND:OPEN?url=https%3A%2F%2Fhome.example.org%2Finvite%2Ftok1`); !ok || link != "https://home.example.org/invite/tok1" {
		t.Fatalf("верхний регистр: %q ok=%v", link, ok)
	}
	// Голый (незакодированный) адрес тоже принимаем: шелл может отдать как есть.
	if link, ok = ParseInviteURL(`wynd:open?url=https://home.example.org/invite/tok1`); !ok || link != "https://home.example.org/invite/tok1" {
		t.Fatalf("незакодированный адрес: %q ok=%v", link, ok)
	}
	// «//open?url=…» — вариант записи шелла.
	if link, ok = ParseInviteURL(`wynd://open?url=https%3A%2F%2Fhome.example.org%2Finvite%2Ftok1`); !ok || link != "https://home.example.org/invite/tok1" {
		t.Fatalf("двойной слэш: %q ok=%v", link, ok)
	}
	// Приглашённый путь без пути — на корень сервера тоже можно выйти.
	if link, ok = ParseInviteURL(`wynd:open?url=https%3A%2F%2Fhome.example.org`); !ok || link != "https://home.example.org/" {
		t.Fatalf("адрес без пути: %q ok=%v", link, ok)
	}

	for name, arg := range map[string]string{
		"чужая схема":     `mailto:a@b.c`,
		"без open":        `wynd:invite/tok`,
		"без url":         `wynd:open?x=1`,
		"http не даём":    `wynd:open?url=http%3A%2F%2Fhome.example.org%2Finvite%2Ftok`,
		"javascript":      `wynd:open?url=javascript%3Aalert(1)`,
		"кривой адрес":    `wynd:open?url=%zz`,
		"пустой параметр": `wynd:open?url=`,
	} {
		if link, ok := ParseInviteURL(arg); ok {
			t.Errorf("%s принято: %q", name, link)
		}
	}
	if _, ok := ParseInviteURL(""); ok {
		t.Error("пустая строка принята")
	}
}
