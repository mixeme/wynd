package installer

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

type memVault map[string]string

func (v memVault) Get(host, user string) (string, bool) {
	p, ok := v[user+"@"+host]
	return p, ok
}

func (v memVault) Set(host, user, password string) error {
	v[user+"@"+host] = password
	return nil
}

func (v memVault) Delete(host, user string) error {
	delete(v, user+"@"+host)
	return nil
}

func newWizard(t *testing.T) (*Wizard, memVault) {
	t.Helper()
	vault := memVault{}
	w := &Wizard{
		Dialer:   Dialer{SSHDir: filepath.Join(t.TempDir(), "ssh")},
		Resolver: fakeResolver{"family.example.ru": {"127.0.0.1"}},
		Vault:    vault,
	}
	t.Cleanup(w.Close)
	return w, vault
}

// Путь первого запуска: отпечаток → подтверждение → осмотр → домен.
func TestWizardFirstConnection(t *testing.T) {
	srv := startServer(t, "pw", nil, cleanUbuntu)
	w, vault := newWizard(t)
	ctx := context.Background()
	in := ConnectInput{Host: srv.host, Port: srv.port, User: "root", Password: "pw"}

	if res := w.Inspect(ctx); res.OK {
		t.Fatal("осмотр без подключения")
	}
	res := w.Connect(ctx, in)
	if res.Status != StatusUnknownHost || !strings.HasPrefix(res.Fingerprint, "SHA256:") {
		t.Fatalf("первое подключение: %+v", res)
	}
	if res := w.Inspect(ctx); res.OK {
		t.Fatal("осмотр до подтверждения отпечатка")
	}
	if res := w.ConfirmHost(ctx); res.Status != StatusConnected {
		t.Fatalf("подтверждение: %+v", res)
	}
	if len(vault) != 0 {
		t.Fatal("пароль сохранён без просьбы")
	}
	insp := w.Inspect(ctx)
	if !insp.OK || insp.Report.Blocked() {
		t.Fatalf("осмотр: %+v", insp)
	}
	// Домен сверяется и с адресом, по которому подключились (127.0.0.1).
	if c := w.CheckDomain(ctx, "family.example.ru"); c.Level != LevelOK {
		t.Fatalf("домен: %+v", c)
	}
	// Второй раз сервер уже знаком.
	if res := w.Connect(ctx, in); res.Status != StatusConnected {
		t.Fatalf("повторное подключение: %+v", res)
	}
}

// «Запомнить пароль»: кладётся после удачного входа, подставляется при пустом
// поле, стирается, когда галочку сняли. Неудачный вход ничего не сохраняет.
func TestWizardRemembersPasswordOnlyOnRequest(t *testing.T) {
	srv := startServer(t, "pw", nil, "")
	w, vault := newWizard(t)
	ctx := context.Background()
	in := ConnectInput{Host: srv.host, Port: srv.port, User: "root", Password: "wrong", Remember: true}
	w.Connect(ctx, in)
	if res := w.ConfirmHost(ctx); res.Status != StatusError || res.Message != "Сервер не принял пароль" {
		t.Fatalf("неверный пароль: %+v", res)
	}
	if len(vault) != 0 || w.HasSavedPassword(srv.host, "root") {
		t.Fatal("сохранён пароль, который не подошёл")
	}

	in.Password = "pw"
	if res := w.Connect(ctx, in); res.Status != StatusConnected {
		t.Fatalf("вход: %+v", res)
	}
	if !w.HasSavedPassword(srv.host, "root") {
		t.Fatal("пароль не сохранён")
	}
	in.Password = ""
	if res := w.Connect(ctx, in); res.Status != StatusConnected {
		t.Fatalf("вход сохранённым паролем: %+v", res)
	}
	in.Password, in.Remember = "pw", false
	if res := w.Connect(ctx, in); res.Status != StatusConnected {
		t.Fatalf("вход без галочки: %+v", res)
	}
	if w.HasSavedPassword(srv.host, "root") {
		t.Fatal("галочку сняли, а пароль остался")
	}
}

func TestWizardExplainsFailures(t *testing.T) {
	w, _ := newWizard(t)
	ctx := context.Background()
	for _, c := range []struct {
		in   ConnectInput
		want string
	}{
		{ConnectInput{Host: "", User: "root", Password: "p"}, "Заполните адрес сервера, пользователя и пароль"},
		{ConnectInput{Host: "127.0.0.1", Port: 1, User: "root", Password: "p"}, "Сервер не отвечает по этому адресу"},
		{ConnectInput{Host: "127.0.0.1", Port: 1, User: "root", UseKeys: true}, "На этом компьютере нет ключа, которым можно войти"},
	} {
		res := w.Connect(ctx, c.in)
		if res.Status != StatusError || res.Message != c.want || res.Advice == "" {
			t.Errorf("%+v: %+v", c.in, res)
		}
	}
}
