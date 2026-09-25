package mail_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"gitea.mixdep.ru/mix/wynd/internal/mail"
)

type liveSMTPFixture struct {
	Name   string
	Config mail.Config
	SendTo string
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("repo root %q: %v", root, err)
	}
	return root
}

func credentialsDir(t *testing.T) string {
	t.Helper()
	if dir := strings.TrimSpace(os.Getenv("WYND_CREDENTIALS_DIR")); dir != "" {
		return dir
	}
	return filepath.Join(repoRoot(t), "dev", "credentials")
}

func loadLiveSMTPFixtures(t *testing.T) []liveSMTPFixture {
	t.Helper()
	var out []liveSMTPFixture
	out = append(out, loadJSONCredentialDir(t)...)

	root := repoRoot(t)
	for _, name := range []string{"credentials.txt", "credentials"} {
		path := filepath.Join(root, "dev", name)
		raw, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		out = append(out, parseCredentialsTxt(t, raw)...)
	}
	if extra := strings.TrimSpace(os.Getenv("WYND_CREDENTIALS_FILE")); extra != "" {
		raw, err := os.ReadFile(extra)
		if err != nil {
			t.Fatalf("WYND_CREDENTIALS_FILE: %v", err)
		}
		out = append(out, parseCredentialsTxt(t, raw)...)
	}

	if len(out) == 0 {
		t.Skipf("smtp live: no credentials (JSON in %s, or dev/credentials.txt; see internal/mail/testdata/)", credentialsDir(t))
	}
	return out
}

func loadJSONCredentialDir(t *testing.T) []liveSMTPFixture {
	t.Helper()
	dir := credentialsDir(t)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []liveSMTPFixture
	for _, e := range entries {
		if e.IsDir() {
			sub := filepath.Join(dir, e.Name())
			subEntries, err := os.ReadDir(sub)
			if err != nil {
				continue
			}
			for _, se := range subEntries {
				if se.IsDir() || !strings.HasSuffix(strings.ToLower(se.Name()), ".json") {
					continue
				}
				out = append(out, parseCredentialFile(t, filepath.Join(sub, se.Name()), se.Name())...)
			}
			continue
		}
		name := e.Name()
		lower := strings.ToLower(name)
		if !strings.HasSuffix(lower, ".json") || strings.Contains(lower, ".example.") {
			continue
		}
		out = append(out, parseCredentialFile(t, filepath.Join(dir, name), name)...)
	}
	return out
}

func parseCredentialsTxt(t *testing.T, raw []byte) []liveSMTPFixture {
	t.Helper()
	var out []liveSMTPFixture
	var sectionTitle string
	fields := map[string]string{}

	flush := func() {
		if len(fields) == 0 {
			return
		}
		fx, ok := txtSectionFixture(sectionTitle, fields)
		if ok {
			out = append(out, fx)
		}
		fields = map[string]string{}
	}

	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#") {
			flush()
			sectionTitle = strings.TrimSpace(strings.TrimLeft(line, "#"))
			continue
		}
		key, val, ok := splitKeyVal(line)
		if !ok {
			continue
		}
		fields[normalizeTxtKey(key)] = val
	}
	flush()
	return out
}

func splitKeyVal(line string) (key, val string, ok bool) {
	i := strings.Index(line, ":")
	if i < 0 {
		return "", "", false
	}
	key = strings.TrimSpace(line[:i])
	val = strings.TrimSpace(line[i+1:])
	return key, val, key != "" && val != ""
}

func normalizeTxtKey(key string) string {
	k := strings.ToLower(strings.TrimSpace(key))
	switch {
	case k == "address", k == "host":
		return "host"
	case strings.Contains(k, "smtp") && strings.Contains(k, "server"):
		return "host"
	case k == "login", strings.Contains(k, "e-mail"):
		return "username"
	case k == "port":
		return "port"
	case k == "password":
		return "password"
	case k == "from":
		return "from"
	case k == "send_to", k == "to":
		return "send_to"
	case k == "name":
		return "name"
	default:
		return k
	}
}

func txtSectionFixture(sectionTitle string, fields map[string]string) (liveSMTPFixture, bool) {
	host := strings.TrimSpace(fields["host"])
	if host == "" {
		return liveSMTPFixture{}, false
	}
	from := strings.TrimSpace(fields["from"])
	username := strings.TrimSpace(fields["username"])
	if from == "" && strings.Contains(username, "@") {
		from = username
	}
	if from == "" {
		return liveSMTPFixture{}, false
	}
	port := 587
	if p := strings.TrimSpace(fields["port"]); p != "" {
		if n, err := strconv.Atoi(p); err == nil && n > 0 {
			port = n
		}
	}
	name := strings.TrimSpace(fields["name"])
	if name == "" {
		name = txtSectionSlug(sectionTitle)
	}
	sendTo := firstNonEmpty(fields["send_to"])
	if sendTo == "" && strings.Contains(username, "@") {
		sendTo = username
	}
	if sendTo == "" {
		sendTo = envelopeAddr(from)
	}
	return liveSMTPFixture{
		Name: name,
		Config: mail.Config{
			Host:     host,
			Port:     port,
			Username: username,
			Password: fields["password"],
			From:     from,
		},
		SendTo: sendTo,
	}, true
}

func txtSectionSlug(title string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return "smtp"
	}
	title = strings.ToLower(title)
	var b strings.Builder
	lastDash := false
	for _, r := range title {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash && b.Len() > 0 {
			b.WriteByte('-')
			lastDash = true
		}
	}
	s := strings.Trim(b.String(), "-")
	if s == "" {
		return "smtp"
	}
	return s
}

func parseCredentialFile(t *testing.T, path, fileName string) []liveSMTPFixture {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return parseCredentialBytes(t, raw, fileName)
}

func parseCredentialBytes(t *testing.T, raw []byte, fileName string) []liveSMTPFixture {
	t.Helper()
	var many []credentialJSON
	if err := json.Unmarshal(raw, &many); err != nil {
		var one credentialJSON
		if err2 := json.Unmarshal(raw, &one); err2 != nil {
			t.Fatalf("parse %s: %v", fileName, err)
		}
		many = []credentialJSON{one}
	}
	var out []liveSMTPFixture
	for i, c := range many {
		fx, ok := c.fixture(fileName, i)
		if !ok {
			continue
		}
		out = append(out, fx)
	}
	return out
}

type credentialJSON struct {
	Name     string          `json:"name"`
	Host     string          `json:"host"`
	Port     int             `json:"port"`
	Username string          `json:"username"`
	Password string          `json:"password"`
	From     string          `json:"from"`
	SendTo   string          `json:"send_to"`
	To       string          `json:"to"`
	SMTP     *credentialJSON `json:"smtp"`
}

func (c credentialJSON) fixture(fileName string, index int) (liveSMTPFixture, bool) {
	if c.SMTP != nil {
		inner := *c.SMTP
		inner.Name = firstNonEmpty(c.Name, inner.Name)
		inner.Host = firstNonEmpty(inner.Host, c.Host)
		inner.Username = firstNonEmpty(inner.Username, c.Username)
		inner.Password = firstNonEmpty(inner.Password, c.Password)
		inner.From = firstNonEmpty(inner.From, c.From)
		if inner.Port <= 0 {
			inner.Port = c.Port
		}
		inner.SendTo = firstNonEmpty(c.SendTo, c.To, inner.SendTo, inner.To)
		c = inner
	}
	host := strings.TrimSpace(c.Host)
	from := strings.TrimSpace(c.From)
	if host == "" || from == "" {
		return liveSMTPFixture{}, false
	}
	name := strings.TrimSpace(c.Name)
	if name == "" {
		base := strings.TrimSuffix(fileName, filepath.Ext(fileName))
		if index > 0 {
			name = base + "-" + strconv.Itoa(index+1)
		} else {
			name = base
		}
	}
	port := c.Port
	if port <= 0 {
		port = 587
	}
	sendTo := firstNonEmpty(c.SendTo, c.To)
	if sendTo == "" {
		sendTo = envelopeAddr(from)
	}
	return liveSMTPFixture{
		Name: name,
		Config: mail.Config{
			Host:     host,
			Port:     port,
			Username: c.Username,
			Password: c.Password,
			From:     from,
		},
		SendTo: sendTo,
	}, true
}

func firstNonEmpty(ss ...string) string {
	for _, s := range ss {
		if strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

func envelopeAddr(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.LastIndex(s, "<"); i >= 0 {
		if j := strings.LastIndex(s, ">"); j > i {
			return strings.TrimSpace(s[i+1 : j])
		}
	}
	return s
}

func TestSMTPCredentialsExampleParses(t *testing.T) {
	path := filepath.Join("testdata", "smtp.credentials.example.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	fixtures := parseCredentialBytes(t, raw, "smtp.credentials.example.json")
	if len(fixtures) != 1 {
		t.Fatalf("fixtures: %d", len(fixtures))
	}
	fx := fixtures[0]
	if fx.Name != "example" || fx.Config.Host != "smtp.example.com" || fx.Config.Port != 587 {
		t.Fatalf("unexpected fixture: %+v", fx)
	}
	if fx.SendTo != "user@example.com" {
		t.Fatalf("send_to: %q", fx.SendTo)
	}
}

func TestEnvelopeAddrFromDisplayName(t *testing.T) {
	if got := envelopeAddr(`Wynd <wynd@home.example.org>`); got != "wynd@home.example.org" {
		t.Fatalf("got %q", got)
	}
}

func TestCredentialsTxtExampleParses(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "credentials.example.txt"))
	if err != nil {
		t.Fatal(err)
	}
	fixtures := parseCredentialsTxt(t, raw)
	if len(fixtures) != 2 {
		t.Fatalf("fixtures: %d", len(fixtures))
	}
	if fixtures[0].Name != "relay-one" || fixtures[0].Config.Port != 587 {
		t.Fatalf("relay: %+v", fixtures[0])
	}
	if fixtures[1].Config.Host != "mail.example.com" || fixtures[1].SendTo != "user@example.com" {
		t.Fatalf("mailbox: %+v", fixtures[1])
	}
}
