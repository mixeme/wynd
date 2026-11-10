package mail

import (
	"context"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/store"
)

func TestLinkifyMakesLinksClickable(t *testing.T) {
	cases := []struct{ in, want string }{
		{
			"Войти — по ссылке:\nhttps://wynd.example.com/move/a1b2",
			"Войти — по ссылке:\n" +
				`<a href="https://wynd.example.com/move/a1b2">https://wynd.example.com/move/a1b2</a>`,
		},
		{
			"Ответить можно в панели:\nhttps://wynd.example.com/admin#requests.",
			"Ответить можно в панели:\n" +
				`<a href="https://wynd.example.com/admin#requests">https://wynd.example.com/admin#requests</a>.`,
		},
		{
			"https://s.example.com/b?a=1&b=2",
			`<a href="https://s.example.com/b?a=1&amp;b=2">https://s.example.com/b?a=1&amp;b=2</a>`,
		},
		{
			"Без адреса — просто текст с <тегом>",
			"Без адреса — просто текст с &lt;тегом&gt;",
		},
		{
			"Слово и https://wynd.example.com в середине строки",
			`Слово и <a href="https://wynd.example.com">https://wynd.example.com</a> в середине строки`,
		},
	}
	for _, tc := range cases {
		if got := linkify(tc.in); got != tc.want {
			t.Fatalf("%q:\n got %q\nwant %q", tc.in, got, tc.want)
		}
	}

	// Через настоящее письмо: абзац со ссылкой в HTML-части становится <a>.
	htmlPart := htmlBody("Тема", "Ответить можно в панели:\nhttps://wynd.example.com/admin#requests.\n")
	if !strings.Contains(htmlPart, "href=") || !strings.Contains(htmlPart, "admin#requests") {
		t.Fatalf("ссылка потерялась в письме:\n%s", htmlPart)
	}
}

func TestEnvelopeAddress(t *testing.T) {
	cases := []struct{ in, want string }{
		{"a@b.com", "a@b.com"},
		{"Wynd <wynd@home.example.org>", "wynd@home.example.org"},
		{` "Name" <x@y.z> `, "x@y.z"},
	}
	for _, tc := range cases {
		if got := envelopeAddress(tc.in); got != tc.want {
			t.Fatalf("%q: got %q want %q", tc.in, got, tc.want)
		}
	}
}

func TestUseImplicitTLS(t *testing.T) {
	if !useImplicitTLS(465) {
		t.Fatal("port 465 must use implicit TLS")
	}
	if useImplicitTLS(587) || useImplicitTLS(25) {
		t.Fatal("STARTTLS ports must not use implicit TLS")
	}
}

func TestSendTimeoutWhenServerSilent(t *testing.T) {
	old := sendTimeout
	sendTimeout = 200 * time.Millisecond
	t.Cleanup(func() { sendTimeout = old })

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		time.Sleep(time.Second)
	}()

	st, err := store.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	svc, err := New(st, false, nil)
	if err != nil {
		t.Fatal(err)
	}

	host, portStr, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	if err := svc.SaveConfig(ctx, Config{
		Host: host,
		Port: port,
		From: "wynd@example.com",
	}); err != nil {
		t.Fatal(err)
	}

	start := time.Now()
	err = svc.SendTest(ctx, "admin@example.com")
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("want timeout, got nil")
	}
	if elapsed > time.Second {
		t.Fatalf("timeout too slow: %s", elapsed)
	}
}
