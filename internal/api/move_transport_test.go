package api

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"reflect"
	"sync/atomic"
	"testing"
)

func TestMoveLinkPathBoundaries(t *testing.T) {
	for _, link := range []string{
		"http://example.com/private%3F/move/AAAA-BBBB",
		"http://example.com/private%23/move/AAAA-BBBB",
		"http://example.com/x/../move/AAAA-BBBB", "http://example.com/move/AAAA-BBBB?q=x",
		"http://example.com/move/AAAA-BBBB#fragment", "http://user@example.com/move/AAAA-BBBB",
	} {
		if _, _, ok := parseMoveLink(link); ok {
			t.Errorf("accepted %s", link)
		}
	}
	base, token, ok := parseMoveLink("https://example.com/wynd/move/AAAA-BBBB")
	if !ok || base != "https://example.com/wynd" || token != "AAAA-BBBB" {
		t.Fatalf("%s %s %v", base, token, ok)
	}
}

func TestMoveAddressPolicy(t *testing.T) {
	for _, address := range []string{"0.0.0.0", "127.0.0.1", "10.0.0.1", "172.16.0.1", "192.168.1.1", "169.254.169.254", "100.100.100.200", "198.18.0.1", "192.0.2.1", "224.0.0.1", "255.255.255.255", "::", "::1", "::ffff:127.0.0.1", "fc00::1", "fe80::1", "fe80::1%eth0", "64:ff9b::a9fe:a9fe", "2001:db8::1", "2002:7f00:1::"} {
		if moveAddressAllowed(netip.MustParseAddr(address), nil) {
			t.Errorf("allowed %s", address)
		}
	}
	for _, address := range []string{"8.8.8.8", "::ffff:8.8.8.8", "2606:4700:4700::1111"} {
		if !moveAddressAllowed(netip.MustParseAddr(address), nil) {
			t.Errorf("blocked %s", address)
		}
	}
	allowed := []netip.Prefix{netip.MustParsePrefix("10.1.2.3/32")}
	if !moveAddressAllowed(netip.MustParseAddr("10.1.2.3"), allowed) || moveAddressAllowed(netip.MustParseAddr("10.1.2.4"), allowed) {
		t.Fatal("LAN exception escaped its network")
	}
}

func TestMoveDialPinsAddressesAndRejectsMixedDNS(t *testing.T) {
	var connected []string
	answers := [][]netip.Addr{
		{netip.MustParseAddr("8.8.8.8")},
		{netip.MustParseAddr("8.8.8.8"), netip.MustParseAddr("127.0.0.1")},
		{netip.MustParseAddr("127.0.0.1")},
	}
	lookups := 0
	dial := guardedMoveDial(nil, func(context.Context, string, string) ([]netip.Addr, error) {
		result := answers[lookups]
		lookups++
		return result, nil
	}, func(_ context.Context, _, address string) (net.Conn, error) {
		connected = append(connected, address)
		return nil, errors.New("test dial")
	})
	for range answers {
		_, _ = dial(t.Context(), "tcp", "source.example:443")
	}
	if lookups != 3 || !reflect.DeepEqual(connected, []string{"8.8.8.8:443"}) {
		t.Fatalf("lookups=%d connected=%v", lookups, connected)
	}
}

func TestMoveRejectsLoopbackBeforeConnection(t *testing.T) {
	var hits atomic.Int32
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits.Add(1); w.WriteHeader(404) }))
	defer source.Close()
	s := &Server{}
	_, _ = s.fetchMoveCard(t.Context(), source.URL+"/move/AAAA-BBBB-CCCC-DDDD")
	_, _ = s.fetchMoveCard(t.Context(), source.URL+"/private%3F/move/AAAA-BBBB-CCCC-DDDD")
	if hits.Load() != 0 {
		t.Fatal("source reached loopback")
	}
}

func TestMoveLANExceptionAndRedirect(t *testing.T) {
	var targetHits atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { targetHits.Add(1) }))
	defer target.Close()
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL, http.StatusFound) }))
	defer source.Close()
	s := &Server{MoveAllowedNetworks: []netip.Prefix{netip.MustParsePrefix("127.0.0.1/32")}}
	resp, err := s.moveCall(t.Context(), source.URL+"/move/AAAA-BBBB", "GET", "/api/v1/move", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusFound || targetHits.Load() != 0 {
		t.Fatal("redirect followed")
	}
}

func TestIncomingWorkerContainsUnexpectedPanic(t *testing.T) {
	// An unexpected fault still must be returned to runIncomingMove for cleanup.
	err := (&Server{}).incomingMoveSafely(t.Context(), "missing")
	var failed *moveFailure
	if !errors.As(err, &failed) || failed.code != "archive_invalid" {
		t.Fatalf("panic escaped cleanup: %v", err)
	}
}
