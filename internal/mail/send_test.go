package mail

import (
	"context"
	"net"
	"strconv"
	"testing"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/store"
)

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
