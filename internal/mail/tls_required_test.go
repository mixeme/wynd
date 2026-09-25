package mail

import (
	"bufio"
	"context"
	"net"
	"strings"
	"testing"

	"gitea.mixdep.ru/mix/wynd/internal/store"
)

// plainSMTPFixture — релей, который не объявляет STARTTLS.
func plainSMTPFixture(t *testing.T) net.Listener {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				w := bufio.NewWriter(conn)
				r := bufio.NewReader(conn)
				write := func(s string) {
					_, _ = w.WriteString(s + "\r\n")
					_ = w.Flush()
				}
				write("220 relay.example ESMTP")
				for {
					line, err := r.ReadString('\n')
					if err != nil {
						return
					}
					cmd := strings.ToUpper(strings.TrimSpace(line))
					switch {
					case strings.HasPrefix(cmd, "EHLO"):
						// Ни STARTTLS, ни AUTH: открытый канал.
						write("250-relay.example")
						write("250 SIZE 10240000")
					case strings.HasPrefix(cmd, "HELO"):
						write("250 relay.example")
					case strings.HasPrefix(cmd, "DATA"):
						write("354 send it")
						for {
							body, err := r.ReadString('\n')
							if err != nil {
								return
							}
							if strings.TrimSpace(body) == "." {
								break
							}
						}
						write("250 queued")
					case strings.HasPrefix(cmd, "QUIT"):
						write("221 bye")
						return
					default:
						write("250 ok")
					}
				}
			}()
		}
	}()
	return ln
}

func newMailService(t *testing.T) *Service {
	t.Helper()
	st, err := store.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	svc, err := New(st, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	return svc
}

// Инвариант (SEC-5): вне localhost письмо не уходит по открытому каналу.
// Посредник может вырезать 250-STARTTLS, и код входа уйдёт открытым текстом.
func TestSendRequiresTLSOutsideLoopback(t *testing.T) {
	ln := plainSMTPFixture(t)
	svc := newMailService(t)
	// Хост релея — не loopback, соединение подменяем на фикстуру.
	svc.dial = func(ctx context.Context, addr string) (net.Conn, error) {
		var d net.Dialer
		return d.DialContext(ctx, "tcp", ln.Addr().String())
	}
	ctx := context.Background()
	if err := svc.SaveConfig(ctx, Config{
		Host: "relay.example", Port: 587, From: "wynd@example.com",
	}); err != nil {
		t.Fatal(err)
	}

	err := svc.SendTest(ctx, "admin@example.com")
	if err == nil {
		t.Fatal("письмо ушло по открытому каналу")
	}
	if !strings.Contains(err.Error(), "STARTTLS required") {
		t.Fatalf("err = %v, want STARTTLS required", err)
	}
}

// Локальный релей без STARTTLS остаётся рабочим: канал не покидает машину.
func TestSendAllowsPlaintextOnLoopbackRelay(t *testing.T) {
	ln := plainSMTPFixture(t)
	svc := newMailService(t)
	host, port := splitFixtureAddr(t, ln)
	ctx := context.Background()
	if err := svc.SaveConfig(ctx, Config{
		Host: host, Port: port, From: "wynd@example.com",
	}); err != nil {
		t.Fatal(err)
	}
	if err := svc.SendTest(ctx, "admin@example.com"); err != nil {
		t.Fatalf("локальный релей: %v", err)
	}
}

// Инвариант (SEC-5): CR/LF в адресе и теме не дописывают своих заголовков.
func TestBuildMessageStripsHeaderInjection(t *testing.T) {
	msg := string(buildMessage(
		"wynd@example.com",
		"victim@example.com\r\nBcc: attacker@evil.example",
		"Код входа\r\nX-Injected: 1",
		"тело",
	))
	head, _, ok := strings.Cut(msg, "\r\n\r\n")
	if !ok {
		t.Fatalf("нет границы заголовков: %q", msg)
	}
	for _, line := range strings.Split(head, "\r\n") {
		for _, bad := range []string{"Bcc:", "X-Injected:"} {
			if strings.HasPrefix(line, bad) {
				t.Fatalf("появился отдельный заголовок %s:\n%s", bad, head)
			}
		}
	}
	if !strings.Contains(head, "victim@example.com") {
		t.Fatalf("адрес получателя потерян:\n%s", head)
	}
}

func splitFixtureAddr(t *testing.T, ln net.Listener) (string, int) {
	t.Helper()
	host, portStr, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	port := 0
	for _, r := range portStr {
		port = port*10 + int(r-'0')
	}
	return host, port
}
