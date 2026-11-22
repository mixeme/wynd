//go:build windows

package desktop

import (
	"strings"
	"testing"
)

// Живой Credential Manager: roundtrip учётной записи окна и отказ на блоб,
// который в неё не влезает. Записи под настоящим префиксом с пробными origin
// убираются в конце.
func TestWincredStoreRoundtrip(t *testing.T) {
	store := NewWincredStore()
	origin := "https://probe.example.org"
	t.Cleanup(func() { store.Delete(origin) })

	blob := []byte(`{"origin":"https://probe.example.org","token":"tok","name":"Аня"}`)
	if err := store.Put(origin, blob); err != nil {
		t.Fatalf("put: %v", err)
	}
	got, ok := store.Get(origin)
	if !ok || string(got) != string(blob) {
		t.Fatalf("read back: ok=%v %q", ok, got)
	}
	found := false
	for _, listed := range store.List() {
		if listed == origin {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("origin нет в перечне")
	}
	if !store.Delete(origin) {
		t.Fatal("delete")
	}
	if _, ok := store.Get(origin); ok {
		t.Fatal("сессия осталась после удаления")
	}
}

func TestWincredStoreRejectsOversizedBlob(t *testing.T) {
	store := NewWincredStore()
	big := []byte(`{"token":"` + strings.Repeat("w", 1200) + `"}`)
	if err := store.Put("https://probe.example.org", big); err == nil {
		store.Delete("https://probe.example.org")
		t.Fatal("большой блоб принят")
	}
}
