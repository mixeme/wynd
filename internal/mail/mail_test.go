package mail_test

import (
	"bufio"
	"context"
	"io"
	"mime"
	"mime/multipart"
	"net"
	netmail "net/mail"
	"strings"
	"testing"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/auth"
	"gitea.mixdep.ru/mix/wynd/internal/mail"
	"gitea.mixdep.ru/mix/wynd/internal/store"
)

func TestConfiguredFalseByDefault(t *testing.T) {
	st, err := store.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	svc, err := mail.New(st, true, auth.NewCaptureCodes())
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	ok, err := svc.Configured(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("want not configured by default")
	}
}

func TestSendCodeLoopbackFallback(t *testing.T) {
	st, err := store.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	caps := auth.NewCaptureCodes()
	svc, err := mail.New(st, true, caps)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	if err := svc.SendCode(ctx, "user@example.com", "123456"); err != nil {
		t.Fatal(err)
	}
	if caps.Last("user@example.com") != "123456" {
		t.Fatalf("code: got %q", caps.Last("user@example.com"))
	}
}

func TestSendCodeNonLoopbackRequiresSMTP(t *testing.T) {
	st, err := store.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	svc, err := mail.New(st, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	err = svc.SendCode(ctx, "user@example.com", "123456")
	if err != mail.ErrNotConfigured {
		t.Fatalf("got %v, want ErrNotConfigured", err)
	}
}

func TestSaveConfigAndSendViaFakeSMTP(t *testing.T) {
	addr, received := startFakeSMTP(t)
	host, port, _ := net.SplitHostPort(addr)

	st, err := store.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	svc, err := mail.New(st, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	if err := svc.SaveConfig(ctx, mail.Config{
		Host: host,
		Port: atoi(port),
		From: "wynd@example.com",
	}); err != nil {
		t.Fatal(err)
	}

	ok, err := svc.Configured(ctx)
	if err != nil || !ok {
		t.Fatalf("configured: ok=%v err=%v", ok, err)
	}

	if err := svc.SendPlain(ctx, "bob@example.com", "Тема", "Текст"); err != nil {
		t.Fatal(err)
	}

	select {
	case msg := <-received:
		if !strings.Contains(msg, "To: bob@example.com") {
			t.Fatalf("missing recipient: %q", msg)
		}
		if !strings.Contains(msg, "Subject:") {
			t.Fatalf("missing subject: %q", msg)
		}
		if !strings.Contains(plainText(t, msg), "Текст") {
			t.Fatalf("missing body: %q", msg)
		}
		// Gmail снимал баллы за 8-битное тело без заголовка, безымянного
		// отправителя и письмо без HTML-части.
		for _, want := range []string{
			"Content-Transfer-Encoding: quoted-printable",
			"multipart/alternative",
			"text/html",
			"Auto-Submitted: auto-generated",
			`From: "Wynd" <wynd@example.com>`,
		} {
			if !strings.Contains(msg, want) {
				t.Fatalf("нет %q в письме: %s", want, msg)
			}
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for smtp message")
	}
}

func TestSendDisplayNameUsesEnvelope(t *testing.T) {
	addr, mailFrom, received := startFakeSMTPCapture(t)
	host, port, _ := net.SplitHostPort(addr)

	st, err := store.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	svc, err := mail.New(st, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	if err := svc.SaveConfig(ctx, mail.Config{
		Host: host,
		Port: atoi(port),
		From: "Wynd <wynd@example.com>",
	}); err != nil {
		t.Fatal(err)
	}

	if err := svc.SendPlain(ctx, "bob@example.com", "Тема", "Текст"); err != nil {
		t.Fatal(err)
	}

	from := <-mailFrom
	if !strings.Contains(from, "<wynd@example.com>") {
		t.Fatalf("MAIL FROM: %q", from)
	}
	if strings.Contains(strings.ToLower(from), "wynd <") {
		t.Fatalf("display name leaked into envelope: %q", from)
	}
	msg := <-received
	if !strings.Contains(msg, "From:") || !strings.Contains(msg, "wynd@example.com") {
		t.Fatalf("From header: %q", msg)
	}
}

func TestProbeViaFakeSMTP(t *testing.T) {
	addr, received := startFakeSMTP(t)
	host, port, _ := net.SplitHostPort(addr)

	st, err := store.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	svc, err := mail.New(st, false, nil)
	if err != nil {
		t.Fatal(err)
	}

	if err := svc.Probe(context.Background(), mail.Config{
		Host: host,
		Port: atoi(port),
		From: "wynd@example.com",
	}); err != nil {
		t.Fatal(err)
	}
	select {
	case msg := <-received:
		t.Fatalf("probe must not send a message: %q", msg)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestProbeRequiresHost(t *testing.T) {
	st, err := store.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	svc, err := mail.New(st, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Probe(context.Background(), mail.Config{From: "wynd@example.com"}); err != mail.ErrNotConfigured {
		t.Fatalf("got %v, want ErrNotConfigured", err)
	}
}

func TestSendTestRecordsTimestamp(t *testing.T) {
	addr, received := startFakeSMTP(t)
	host, port, _ := net.SplitHostPort(addr)

	st, err := store.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	svc, err := mail.New(st, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	if err := svc.SaveConfig(ctx, mail.Config{
		Host: host,
		Port: atoi(port),
		From: "wynd@example.com",
	}); err != nil {
		t.Fatal(err)
	}

	if err := svc.SendTest(ctx, "admin@example.com"); err != nil {
		t.Fatal(err)
	}
	<-received

	cfg, err := svc.LoadConfig(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.TestSentAt == nil {
		t.Fatal("want test_sent_at set")
	}
	if cfg.LastError != "" {
		t.Fatalf("last error: %q", cfg.LastError)
	}
}

func startFakeSMTP(t *testing.T) (addr string, received chan string) {
	t.Helper()
	addr, _, received = startFakeSMTPCapture(t)
	return addr, received
}

func startFakeSMTPCapture(t *testing.T) (addr string, mailFrom, received chan string) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })

	mailFrom = make(chan string, 1)
	received = make(chan string, 1)
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go handleSMTPConn(conn, mailFrom, received)
		}
	}()
	return ln.Addr().String(), mailFrom, received
}

func handleSMTPConn(conn net.Conn, mailFrom, received chan<- string) {
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	writeLine := func(s string) { _, _ = conn.Write([]byte(s)) }
	readLine := func() string {
		line, _, _ := bufio.NewReader(conn).ReadLine()
		return string(line)
	}

	writeLine("220 fake.test ESMTP\r\n")
	for {
		line := readLine()
		upper := strings.ToUpper(line)
		switch {
		case strings.HasPrefix(upper, "EHLO"), strings.HasPrefix(upper, "HELO"):
			writeLine("250-fake.test\r\n250 OK\r\n")
		case strings.HasPrefix(upper, "STARTTLS"):
			writeLine("220 Ready to start TLS\r\n")
		case strings.HasPrefix(upper, "MAIL FROM"):
			select {
			case mailFrom <- line:
			default:
			}
			writeLine("250 OK\r\n")
		case strings.HasPrefix(upper, "RCPT TO"):
			writeLine("250 OK\r\n")
		case strings.HasPrefix(upper, "DATA"):
			writeLine("354 End data with <CR><LF>.<CR><LF>\r\n")
			var body strings.Builder
			r := bufio.NewReader(conn)
			for {
				l, err := r.ReadString('\n')
				if err != nil {
					break
				}
				if l == ".\r\n" || l == ".\n" {
					break
				}
				body.WriteString(l)
			}
			received <- body.String()
			writeLine("250 OK\r\n")
		case strings.HasPrefix(upper, "QUIT"):
			writeLine("221 Bye\r\n")
			return
		case strings.HasPrefix(upper, "AUTH"):
			writeLine("235 Authentication successful\r\n")
		default:
			writeLine("250 OK\r\n")
		}
	}
}

func atoi(s string) int {
	n := 0
	for _, c := range s {
		n = n*10 + int(c-'0')
	}
	return n
}

// plainText — текстовая часть письма, раскодированная из quoted-printable.
func plainText(t *testing.T, raw string) string {
	t.Helper()
	m, err := netmail.ReadMessage(strings.NewReader(raw))
	if err != nil {
		t.Fatalf("письмо не разбирается: %v", err)
	}
	_, params, err := mime.ParseMediaType(m.Header.Get("Content-Type"))
	if err != nil {
		t.Fatal(err)
	}
	r := multipart.NewReader(m.Body, params["boundary"])
	for {
		part, err := r.NextPart()
		if err == io.EOF {
			t.Fatal("нет text/plain")
		}
		if err != nil {
			t.Fatal(err)
		}
		if strings.HasPrefix(part.Header.Get("Content-Type"), "text/plain") {
			body, err := io.ReadAll(part)
			if err != nil {
				t.Fatal(err)
			}
			return string(body)
		}
	}
}

// Код — в тексте письма, но не в теме: тема видна в уведомлении на
// заблокированном экране, код оттуда прочитал бы любой, кто рядом.
func TestCodeMailKeepsCodeOutOfSubject(t *testing.T) {
	addr, received := startFakeSMTP(t)
	host, port, _ := net.SplitHostPort(addr)
	st, err := store.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	svc, err := mail.New(st, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := svc.SaveConfig(ctx, mail.Config{Host: host, Port: atoi(port), From: "wynd@example.com"}); err != nil {
		t.Fatal(err)
	}
	if err := svc.SendCode(ctx, "bob@example.com", "276012"); err != nil {
		t.Fatal(err)
	}
	select {
	case msg := <-received:
		m, err := netmail.ReadMessage(strings.NewReader(msg))
		if err != nil {
			t.Fatal(err)
		}
		subject, err := new(mime.WordDecoder).DecodeHeader(m.Header.Get("Subject"))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(subject, "276012") {
			t.Fatalf("код в теме: %q", subject)
		}
		if !strings.Contains(plainText(t, msg), "276012") {
			t.Fatalf("кода нет в тексте: %q", msg)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for smtp message")
	}
}
