package installer

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

// testServer — подставной SSH-сервер: принимает один пароль или один ключ и
// на любую команду отвечает заготовленным выводом.
type testServer struct {
	addr     string
	host     string
	port     int
	commands chan string
	// handler, если задан, отвечает на команду сам: вывод и код выхода.
	// stdin — то, что пришло на вход команды (файл, образ).
	handler func(command string, stdin []byte) (string, int)
}

func startServer(t *testing.T, password string, authorized ssh.PublicKey, output string) *testServer {
	t.Helper()
	_, hostPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	hostSigner, err := ssh.NewSignerFromKey(hostPriv)
	if err != nil {
		t.Fatal(err)
	}
	cfg := &ssh.ServerConfig{
		PasswordCallback: func(_ ssh.ConnMetadata, pass []byte) (*ssh.Permissions, error) {
			if password != "" && string(pass) == password {
				return nil, nil
			}
			return nil, errors.New("denied")
		},
		PublicKeyCallback: func(_ ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			if authorized != nil && string(key.Marshal()) == string(authorized.Marshal()) {
				return nil, nil
			}
			return nil, errors.New("denied")
		},
	}
	cfg.AddHostKey(hostSigner)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	srv := &testServer{addr: ln.Addr().String(), commands: make(chan string, 8)}
	tcp := ln.Addr().(*net.TCPAddr)
	srv.host, srv.port = "127.0.0.1", tcp.Port
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go srv.serve(conn, cfg, output)
		}
	}()
	return srv
}

func (s *testServer) serve(conn net.Conn, cfg *ssh.ServerConfig, output string) {
	defer func() { _ = conn.Close() }()
	_, chans, reqs, err := ssh.NewServerConn(conn, cfg)
	if err != nil {
		return
	}
	go ssh.DiscardRequests(reqs)
	for newCh := range chans {
		if newCh.ChannelType() != "session" {
			_ = newCh.Reject(ssh.UnknownChannelType, "no")
			continue
		}
		ch, chReqs, err := newCh.Accept()
		if err != nil {
			return
		}
		go func() {
			defer func() { _ = ch.Close() }()
			for req := range chReqs {
				if req.Type != "exec" {
					_ = req.Reply(false, nil)
					continue
				}
				var payload struct{ Command string }
				_ = ssh.Unmarshal(req.Payload, &payload)
				_ = req.Reply(true, nil)
				if s.handler != nil {
					stdin, _ := io.ReadAll(ch)
					out, code := s.handler(payload.Command, stdin)
					_, _ = ch.Write([]byte(out))
					_, _ = ch.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{uint32(code)}))
					return
				}
				s.commands <- payload.Command
				_, _ = ch.Write([]byte(output))
				_, _ = ch.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{0}))
				return
			}
		}()
	}
}

func (s *testServer) access(password string) Access {
	return Access{Host: s.host, Port: s.port, User: "root", Password: password}
}

// Первое подключение: без подтверждения отпечатка входа нет и в known_hosts
// ничего не пишется; после подтверждения — вход, а подмена ключа заметна.
func TestDialAsksAboutUnknownHostThenRemembers(t *testing.T) {
	srv := startServer(t, "s3cret", nil, "hello\n")
	d := Dialer{SSHDir: filepath.Join(t.TempDir(), "ssh")}
	ctx := context.Background()

	_, err := d.Dial(ctx, srv.access("s3cret"))
	var unknown *UnknownHostError
	if !errors.As(err, &unknown) {
		t.Fatalf("первое подключение: %v", err)
	}
	if !strings.HasPrefix(unknown.Fingerprint, "SHA256:") {
		t.Fatalf("отпечаток: %q", unknown.Fingerprint)
	}
	if _, statErr := os.Stat(filepath.Join(d.SSHDir, "known_hosts")); statErr == nil {
		t.Fatal("known_hosts записан без подтверждения")
	}
	select {
	case cmd := <-srv.commands:
		t.Fatalf("до подтверждения на сервере выполнено: %q", cmd)
	default:
	}

	if err := d.Trust(unknown); err != nil {
		t.Fatalf("Trust: %v", err)
	}
	sess, err := d.Dial(ctx, srv.access("s3cret"))
	if err != nil {
		t.Fatalf("после подтверждения: %v", err)
	}
	defer func() { _ = sess.Close() }()
	out, err := sess.Run(ctx, "echo hello")
	if err != nil || out != "hello\n" {
		t.Fatalf("Run: %q %v", out, err)
	}
	if sess.RemoteIP() != "127.0.0.1" {
		t.Fatalf("RemoteIP: %q", sess.RemoteIP())
	}

}

func TestDialRefusesChangedHostKey(t *testing.T) {
	first := startServer(t, "pw", nil, "")
	d := Dialer{SSHDir: filepath.Join(t.TempDir(), "ssh")}
	ctx := context.Background()
	_, err := d.Dial(ctx, first.access("pw"))
	var unknown *UnknownHostError
	if !errors.As(err, &unknown) {
		t.Fatalf("первое подключение: %v", err)
	}
	if err := d.Trust(unknown); err != nil {
		t.Fatal(err)
	}
	// Другой сервер (другой ключ) под запомненным именем: переписываем
	// запись known_hosts на его адрес, как было бы при подмене.
	second := startServer(t, "pw", nil, "")
	path := filepath.Join(d.SSHDir, "known_hosts")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	rewritten := strings.ReplaceAll(string(raw), first.addr, second.addr)
	rewritten = strings.ReplaceAll(rewritten, "["+first.host+"]:"+strconv.Itoa(first.port), "["+second.host+"]:"+strconv.Itoa(second.port))
	if rewritten == string(raw) {
		t.Fatalf("запись known_hosts не найдена: %q", raw)
	}
	if err := os.WriteFile(path, []byte(rewritten), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = d.Dial(ctx, second.access("pw"))
	var changed *ChangedHostError
	if !errors.As(err, &changed) {
		t.Fatalf("подменённый ключ: %v", err)
	}
	select {
	case cmd := <-second.commands:
		t.Fatalf("на подменённом сервере выполнено: %q", cmd)
	default:
	}
}

func trusted(t *testing.T, d Dialer, a Access) {
	t.Helper()
	_, err := d.Dial(context.Background(), Access{Host: a.Host, Port: a.Port, User: a.User, Password: "x"})
	var unknown *UnknownHostError
	if !errors.As(err, &unknown) {
		t.Fatalf("ожидали вопрос об отпечатке: %v", err)
	}
	if err := d.Trust(unknown); err != nil {
		t.Fatal(err)
	}
}

func TestDialRefusals(t *testing.T) {
	srv := startServer(t, "right", nil, "")
	d := Dialer{SSHDir: filepath.Join(t.TempDir(), "ssh")}
	ctx := context.Background()
	trusted(t, d, srv.access(""))

	if _, err := d.Dial(ctx, srv.access("wrong")); !errors.Is(err, ErrAuth) {
		t.Fatalf("неверный пароль: %v", err)
	}
	if _, err := d.Dial(ctx, srv.access("")); !errors.Is(err, ErrInvalid) {
		t.Fatalf("пустой пароль: %v", err)
	}
	for _, host := range []string{"", "root@1.2.3.4", "ssh://1.2.3.4", "1.2.3.4 -oProxyCommand=x"} {
		if _, err := d.Dial(ctx, Access{Host: host, User: "root", Password: "p"}); !errors.Is(err, ErrInvalid) {
			t.Fatalf("адрес %q: %v", host, err)
		}
	}
	// Никто не слушает — «сервер не отвечает», а не отказ в доступе.
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	if _, err := d.Dial(ctx, Access{Host: "127.0.0.1", Port: port, User: "root", Password: "p"}); !errors.Is(err, ErrUnreachable) {
		t.Fatalf("закрытый порт: %v", err)
	}
	if _, err := d.Dial(ctx, Access{Host: srv.host, Port: srv.port, User: "root", UseKeys: true}); !errors.Is(err, ErrNoKeys) {
		t.Fatalf("без ключей: %v", err)
	}
}

func TestDialWithKeyFromSSHDir(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	sshPub, err := ssh.NewPublicKey(pub)
	if err != nil {
		t.Fatal(err)
	}
	block, err := ssh.MarshalPrivateKey(priv, "")
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "ssh")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "id_ed25519"), pem.EncodeToMemory(block), 0o600); err != nil {
		t.Fatal(err)
	}
	srv := startServer(t, "", sshPub, "ok\n")
	d := Dialer{SSHDir: dir}
	trusted(t, d, srv.access(""))
	sess, err := d.Dial(context.Background(), Access{Host: srv.host, Port: srv.port, User: "root", UseKeys: true})
	if err != nil {
		t.Fatalf("вход ключом: %v", err)
	}
	_ = sess.Close()
}

// Запись в known_hosts без перевода строки в конце не должна слипнуться с новой.
func TestTrustKeepsExistingKnownHostsLines(t *testing.T) {
	srv := startServer(t, "pw", nil, "")
	dir := filepath.Join(t.TempDir(), "ssh")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "known_hosts")
	existing := "other.example ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIJ7yVZL0gkW0vVQH2Vw1u3o4mX1cM9o0o3m3m3m3m3m3"
	if err := os.WriteFile(path, []byte(existing), 0o600); err != nil {
		t.Fatal(err)
	}
	d := Dialer{SSHDir: dir}
	trusted(t, d, srv.access(""))
	raw, _ := os.ReadFile(path)
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) != 2 || lines[0] != existing {
		t.Fatalf("known_hosts: %q", raw)
	}
}

const cleanUbuntu = `## os
PRETTY_NAME="Ubuntu 24.04.1 LTS"
NAME="Ubuntu"
VERSION_ID="24.04"
ID=ubuntu
## arch
x86_64
## uid
0
## sudo
## disk
/dev/vda1       41152736 8234124  32901228      21% /
## time
1791460000
## ntp
yes
## docker
## compose
## ports
LISTEN 0      4096   127.0.0.53%lo:53        0.0.0.0:*    users:(("systemd-resolve",pid=511,fd=15))
LISTEN 0      128          0.0.0.0:22        0.0.0.0:*    users:(("sshd",pid=702,fd=3))
## addr
2: eth0    inet 185.12.34.56/24 brd 185.12.34.255 scope global eth0\       valid_lft forever preferred_lft forever
## wynd
`

func findings(rep Report) map[string]Finding {
	res := map[string]Finding{}
	for _, f := range rep.Findings {
		res[f.ID] = f
	}
	return res
}

func TestParseInspectionCleanServer(t *testing.T) {
	rep := ParseInspection(cleanUbuntu, "root", time.Unix(1791460003, 0))
	if rep.Blocked() {
		t.Fatalf("чистый сервер не должен мешать: %+v", rep.Findings)
	}
	f := findings(rep)
	if f["os"].Level != LevelOK || f["os"].Text != "Ubuntu 24.04 — подходит" {
		t.Fatalf("os: %+v", f["os"])
	}
	if f["disk"].Text != "Свободно 31 ГБ" {
		t.Fatalf("disk: %+v", f["disk"])
	}
	if f["clock"].Level != LevelOK {
		t.Fatalf("clock: %+v", f["clock"])
	}
	if f["docker"].Level != LevelNote || f["docker"].Plan != "поставим" {
		t.Fatalf("docker: %+v", f["docker"])
	}
	if f["ports"].Level != LevelOK || rep.Proxy != "" {
		t.Fatalf("ports: %+v proxy=%q", f["ports"], rep.Proxy)
	}
	if _, ok := f["root"]; ok {
		t.Fatal("про права молчим, пока их хватает")
	}
	if len(rep.Addresses) != 1 || rep.Addresses[0] != "185.12.34.56" {
		t.Fatalf("addresses: %v", rep.Addresses)
	}
}

func replaceSection(out, name, body string) string {
	start := strings.Index(out, "## "+name+"\n")
	if start < 0 {
		panic("no section " + name)
	}
	rest := out[start+len(name)+4:]
	end := strings.Index(rest, "## ")
	if end < 0 {
		end = len(rest)
	}
	return out[:start] + "## " + name + "\n" + body + rest[end:]
}

func TestParseInspectionFindings(t *testing.T) {
	now := time.Unix(1791460000, 0)
	cases := []struct {
		name    string
		section string
		body    string
		id      string
		level   Level
		text    string
	}{
		{"чужая система", "os", "NAME=\"AlmaLinux\"\nVERSION_ID=\"9.4\"\nID=almalinux\n", "os", LevelBlock, "На сервере AlmaLinux 9.4"},
		{"система неизвестна", "os", "", "os", LevelBlock, "Не удалось узнать, какая система стоит на сервере"},
		{"не тот процессор", "arch", "armv7l\n", "os", LevelBlock, "Процессор сервера (armv7l) не подходит"},
		{"мало места", "disk", "/dev/vda1 10000000 8000000 2000000 80% /\n", "disk", LevelBlock, "Свободно 1,9 ГБ — этого мало"},
		{"часы отстают", "time", "1791459400\n", "clock", LevelNote, "Часы сервера расходятся с вашими на 10 мин"},
		{"часы не сверяются", "ntp", "no\n", "clock", LevelNote, "Часы идут верно, но сами не сверяются"},
		{"docker без compose", "docker", "Docker version 20.10.5\n", "docker", LevelNote, "Docker есть, но без Compose"},
		{"nginx на портах", "ports",
			"LISTEN 0 511 0.0.0.0:80 0.0.0.0:* users:((\"nginx\",pid=1,fd=6),(\"nginx\",pid=2,fd=6))\nLISTEN 0 511 [::]:443 [::]:* users:((\"nginx\",pid=1,fd=7))\n",
			"ports", LevelNote, "Уже работает nginx и занимает порты 80 и 443"},
		{"apache только на 80", "ports", "LISTEN 0 511 *:80 *:* users:((\"apache2\",pid=1,fd=4))\n", "ports", LevelNote, "Уже работает apache и занимает порт 80"},
		{"чужая программа", "ports", "LISTEN 0 4096 0.0.0.0:443 0.0.0.0:* users:((\"docker-proxy\",pid=9,fd=4))\n", "ports", LevelBlock, "Порт 443 на сервере уже занят: docker-proxy"},
		{"два веб-сервера", "ports",
			"LISTEN 0 511 0.0.0.0:80 0.0.0.0:* users:((\"nginx\",pid=1,fd=6))\nLISTEN 0 511 0.0.0.0:443 0.0.0.0:* users:((\"caddy\",pid=3,fd=6))\n",
			"ports", LevelBlock, "Порты 80 и 443 на сервере уже заняты: caddy, nginx"},
		{"wynd уже стоит", "wynd", "/opt/wynd/compose.yaml\n", "wynd", LevelNote, "Wynd на этом сервере уже стоит"},
	}
	for _, c := range cases {
		rep := ParseInspection(replaceSection(cleanUbuntu, c.section, c.body), "root", now)
		f, ok := findings(rep)[c.id]
		if !ok || f.Level != c.level || f.Text != c.text {
			t.Errorf("%s: %+v", c.name, f)
		}
		if f.Level == LevelBlock && f.Advice == "" {
			t.Errorf("%s: помеха без совета, что делать", c.name)
		}
		if f.Level == LevelNote && f.Plan == "" {
			t.Errorf("%s: замечание без того, что с ним сделаем", c.name)
		}
	}
}

func TestParseInspectionNeedsRoot(t *testing.T) {
	out := replaceSection(cleanUbuntu, "uid", "1000\n")
	rep := ParseInspection(out, "anya", time.Unix(1791460000, 0))
	f := findings(rep)["root"]
	if f.Level != LevelBlock || !strings.Contains(f.Text, "anya") {
		t.Fatalf("root: %+v", f)
	}
	// С sudo без пароля — хватает.
	rep = ParseInspection(replaceSection(out, "sudo", "yes\n"), "anya", time.Unix(1791460000, 0))
	if _, ok := findings(rep)["root"]; ok {
		t.Fatal("sudo без пароля достаточно")
	}
}

// Сервер, ответивший мусором, — помеха словами, а не паника.
func TestParseInspectionGarbage(t *testing.T) {
	for _, out := range []string{"", "## os\n## disk\nx\n## ports\n: : :\n## time\nnope\n", "\x00\x01## ##\n## "} {
		rep := ParseInspection(out, "root", time.Now())
		if !rep.Blocked() {
			t.Fatalf("мусор %q не должен выглядеть как годный сервер", out)
		}
	}
}

func TestInspectRunsOneReadOnlyCommand(t *testing.T) {
	srv := startServer(t, "pw", nil, cleanUbuntu)
	d := Dialer{SSHDir: filepath.Join(t.TempDir(), "ssh")}
	trusted(t, d, srv.access(""))
	sess, err := d.Dial(context.Background(), srv.access("pw"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sess.Close() }()
	rep, err := Inspect(context.Background(), sess)
	if err != nil {
		t.Fatal(err)
	}
	if findings(rep)["os"].Level != LevelOK {
		t.Fatalf("os: %+v", rep.Findings)
	}
	cmd := <-srv.commands
	if cmd != inspectScript {
		t.Fatal("на сервер ушла не команда осмотра")
	}
	// Осмотр только читает: в команде нет ничего, что пишет или ставит.
	for _, word := range []string{"rm ", "apt", "install", "systemctl", " > /", ">>", "tee ", "chmod", "mkdir", "curl", "wget"} {
		if strings.Contains(inspectScript, word) {
			t.Errorf("в команде осмотра есть %q", word)
		}
	}
}

type fakeResolver map[string][]string

func (f fakeResolver) LookupHost(_ context.Context, host string) ([]string, error) {
	if a, ok := f[host]; ok {
		return a, nil
	}
	return nil, errors.New("no such host")
}

func TestCheckDomain(t *testing.T) {
	r := fakeResolver{
		"family.example.ru": {"185.12.34.56"},
		"other.example.ru":  {"203.0.113.9"},
	}
	server := []string{"10.0.0.5", "185.12.34.56"}
	ctx := context.Background()

	if c := CheckDomain(ctx, r, " https://Family.Example.ru/ ", server); c.Level != LevelOK || c.Domain != "family.example.ru" {
		t.Fatalf("свой домен: %+v", c)
	}
	c := CheckDomain(ctx, r, "other.example.ru", server)
	if c.Level != LevelBlock || c.Text != "other.example.ru ещё не указывает на этот сервер" ||
		!strings.Contains(c.Advice, "185.12.34.56") || !strings.Contains(c.Advice, "203.0.113.9") {
		t.Fatalf("чужой адрес: %+v", c)
	}
	c = CheckDomain(ctx, r, "new.example.ru", server)
	if c.Level != LevelBlock || c.Text != "new.example.ru пока никуда не указывает" {
		t.Fatalf("нет записи: %+v", c)
	}
	// Резолвер недоступен — виноват не домен.
	c = CheckDomain(ctx, brokenResolver{}, "family.example.ru", server)
	if c.Level != LevelBlock || c.Text != "Не удалось проверить адрес сайта" {
		t.Fatalf("нет связи с DNS: %+v", c)
	}
	for _, raw := range []string{"", "localhost", "185.12.34.56", "a b.ru", "-x.example.ru", "exa$mple.ru"} {
		if c := CheckDomain(ctx, r, raw, server); c.Level != LevelBlock || c.Text != "Это не похоже на адрес сайта" {
			t.Fatalf("%q: %+v", raw, c)
		}
	}
}

type brokenResolver struct{}

func (brokenResolver) LookupHost(context.Context, string) ([]string, error) {
	return nil, &net.DNSError{Err: "server misbehaving", Name: "family.example.ru", IsTemporary: true}
}
