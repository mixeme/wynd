package desktop

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizeAddress(t *testing.T) {
	cases := []struct {
		in, want string
		bad      bool
	}{
		{"home.example.org", "https://home.example.org", false},
		{"  https://Home.Example.org/ ", "https://home.example.org", false},
		{"http://127.0.0.1:7690", "http://127.0.0.1:7690", false},
		{"home.example.org:8443", "https://home.example.org:8443", false},
		{"", "", true},
		{"https://home.example.org/circles", "", true},
		{"https://home.example.org?x=1", "", true},
		{"https://user:pass@home.example.org", "", true},
		{"ftp://home.example.org", "", true},
		{"javascript:alert(1)", "", true},
	}
	for _, c := range cases {
		got, err := NormalizeAddress(c.in)
		if c.bad {
			if !errors.Is(err, ErrInvalidAddress) {
				t.Errorf("%q: want ErrInvalidAddress, got %q, %v", c.in, got, err)
			}
			continue
		}
		if err != nil {
			t.Errorf("%q: unexpected error %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("%q: got %q, want %q", c.in, got, c.want)
		}
	}
}

func TestCheckServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/instance" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{
			"name":              "Дом Ани",
			"version":           "0.50.0",
			"registration_mode": "invite",
		})
	}))
	defer srv.Close()

	got, err := CheckServer(context.Background(), srv.Client(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if got.Origin != srv.URL || got.Name != "Дом Ани" || got.Version != "0.50.0" || got.RegistrationMode != "invite" {
		t.Fatalf("unexpected result: %+v", got)
	}
}

func TestCheckServerRejectsForeignAnswer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("<html>чужой сайт</html>"))
	}))
	defer srv.Close()

	if _, err := CheckServer(context.Background(), srv.Client(), srv.URL); !errors.Is(err, ErrNoServer) {
		t.Fatalf("want ErrNoServer, got %v", err)
	}
}

func TestStorePersistsSelectionAndRename(t *testing.T) {
	path := filepath.Join(t.TempDir(), "servers.json")
	store, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	first := Server{Origin: "https://a.example", Name: "Первый", Interface: true}
	second := Server{Origin: "https://b.example", Name: "Второй", Interface: true}
	if err := store.Put(first, true); err != nil {
		t.Fatal(err)
	}
	if err := store.Put(second, false); err != nil {
		t.Fatal(err)
	}
	selected, list := store.Snapshot()
	if selected != first.Origin || len(list) != 2 {
		t.Fatalf("selected %q, list %+v", selected, list)
	}

	reopened, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	selected, list = reopened.Snapshot()
	if selected != first.Origin || len(list) != 2 || list[0].Name != "Первый" {
		t.Fatalf("after reopen: selected %q, list %+v", selected, list)
	}

	first.Name = "Переименован"
	if err := reopened.Put(first, false); err != nil {
		t.Fatal(err)
	}
	if err := reopened.Select(second.Origin); err != nil {
		t.Fatal(err)
	}
	if err := reopened.Remove(second.Origin); err != nil {
		t.Fatal(err)
	}
	selected, list = reopened.Snapshot()
	if selected != first.Origin || len(list) != 1 || list[0].Name != "Переименован" {
		t.Fatalf("after remove: selected %q, list %+v", selected, list)
	}
	if err := reopened.ClearSelection(); err != nil {
		t.Fatal(err)
	}
	selected, list = reopened.Snapshot()
	if selected != "" || len(list) != 1 {
		t.Fatalf("after clear: selected %q, list %+v", selected, list)
	}
}

func TestStoreInterfaceRoleIsIndependent(t *testing.T) {
	store, err := OpenStore(filepath.Join(t.TempDir(), "servers.json"))
	if err != nil {
		t.Fatal(err)
	}
	current := Server{Origin: "https://current.example", Interface: true}
	account := Server{Origin: "https://account.example", Interface: true, Account: true}
	pure := Server{Origin: "https://pure.example", Interface: true}
	for _, server := range []Server{current, account, pure} {
		if err := store.Put(server, server.Origin == current.Origin); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.RemoveInterface(current.Origin); !errors.Is(err, ErrSelectedServer) {
		t.Fatalf("remove selected: %v", err)
	}
	if err := store.RemoveInterface(account.Origin); err != nil {
		t.Fatal(err)
	}
	if err := store.RemoveInterface(pure.Origin); err != nil {
		t.Fatal(err)
	}
	selected, list := store.Snapshot()
	if selected != current.Origin || len(list) != 2 {
		t.Fatalf("selected %q, list %+v", selected, list)
	}
	if list[1].Origin != account.Origin || list[1].Interface || !list[1].Account {
		t.Fatalf("account role changed: %+v", list[1])
	}
	if err := store.Select(account.Origin); !errors.Is(err, ErrUnknownServer) {
		t.Fatalf("select account-only server: %v", err)
	}
}

func TestStoreMigratesLegacyRoles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "servers.json")
	legacy := []byte(`{"selected":"https://home.example","servers":[{"origin":"https://home.example","name":"Дом Ани"}]}`)
	if err := os.WriteFile(path, legacy, 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	selected, list := store.Snapshot()
	if selected != "https://home.example" || len(list) != 1 || !list[0].Interface || !list[0].Account {
		t.Fatalf("legacy roles: selected %q, list %+v", selected, list)
	}
}
