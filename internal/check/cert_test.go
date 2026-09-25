package check_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/check"
)

func certRows(t *testing.T, info *check.TLSInfo, now time.Time) (le, chain check.Result) {
	t.Helper()
	// RunChecks заодно ходит в DNS и NTP; строкам сертификата сеть не нужна,
	// поэтому ей даётся секунда, чтобы тест не ждал резолвер.
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	for _, r := range check.RunChecks(ctx, check.Input{
		PublicURL:      "https://home.example.org",
		DataDir:        t.TempDir(),
		ServerEgressIP: "203.0.113.7",
		TLS:            info,
		Now:            now,
	}) {
		switch r.ID {
		case "cert_le":
			le = r
		case "cert_chain":
			chain = r
		}
	}
	return le, chain
}

// Инвариант (план 42, CHK-3): предупреждение о сроке — по доле срока жизни,
// а не по 30 дням: у исправного 45-дневного сертификата строка зелёная,
// у сертификата, чьё продление явно не прошло, — жёлтая; чужой доверенный
// центр — не повод для предупреждения; догадки по CN нет.
func TestCertExpiryWarnsByLifetimeShare(t *testing.T) {
	now := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)
	le := func(notBefore time.Time, lifetimeDays int) *check.TLSInfo {
		return &check.TLSInfo{
			NotBefore: notBefore, NotAfter: notBefore.Add(time.Duration(lifetimeDays) * 24 * time.Hour),
			Issuer: "E7", IssuerOrg: "Let's Encrypt", FullChain: true,
		}
	}
	// 45-дневный, осталось 20 дней (> 45/4) — всё в порядке.
	if r, _ := certRows(t, le(now.Add(-25*24*time.Hour), 45), now); r.Status != check.StatusOK {
		t.Fatalf("45-day, 20 left: %+v", r)
	}
	// 90-дневный, осталось 10 дней (< 90/4) — продление не прошло.
	if r, _ := certRows(t, le(now.Add(-80*24*time.Hour), 90), now); r.Status != check.StatusWarn {
		t.Fatalf("90-day, 10 left: %+v", r)
	}
	other := &check.TLSInfo{
		NotBefore: now.Add(-10 * 24 * time.Hour), NotAfter: now.Add(80 * 24 * time.Hour),
		Issuer: "R10", IssuerOrg: "ZeroSSL", FullChain: true,
	}
	r, _ := certRows(t, other, now)
	if r.Status != check.StatusOK || !strings.Contains(r.Detail, "ZeroSSL") {
		t.Fatalf("other CA: %+v", r)
	}
}

// Инвариант (CHK-2): отказ проверки сертификата раскладывается по видам и
// доходит до строк панели, а не превращается в «не удалось прочитать».
func TestCertVerifyErrorsReachRows(t *testing.T) {
	now := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)
	base := check.TLSInfo{NotBefore: now.Add(-24 * time.Hour), NotAfter: now.Add(60 * 24 * time.Hour), Issuer: "x"}
	for _, tc := range []struct {
		verify     check.VerifyError
		le, chain  check.Status
		leContains string
	}{
		{check.VerifySelfSigned, check.StatusFail, check.StatusFail, "самоподписанный"},
		{check.VerifyHostname, check.StatusFail, check.StatusWarn, "не для этого домена"},
		{check.VerifyUnknownAuthority, check.StatusOK, check.StatusFail, ""},
	} {
		info := base
		info.Verify = tc.verify
		le, chain := certRows(t, &info, now)
		if le.Status != tc.le || chain.Status != tc.chain || !strings.Contains(le.Detail, tc.leContains) {
			t.Fatalf("%s: le=%+v chain=%+v", tc.verify, le, chain)
		}
	}
}

// Инвариант (CHK-2, CHK-4): живая проба на самоподписанный сервер на
// нестандартном порту — сертификат прочитан, вид отказа «самоподписанный»,
// порт взят из адреса.
func TestProbeTLSClassifiesSelfSigned(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "localhost"},
		DNSNames:     []string{"localhost"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	srv.TLS = &tls.Config{Certificates: []tls.Certificate{{Certificate: [][]byte{der}, PrivateKey: key}}}
	srv.StartTLS()
	defer srv.Close()
	_, port, err := net.SplitHostPort(strings.TrimPrefix(srv.URL, "https://"))
	if err != nil {
		t.Fatal(err)
	}
	info := check.ProbeTLS(t.Context(), "https://localhost:"+port)
	if info.ProbeError != "" || info.Verify != check.VerifySelfSigned {
		t.Fatalf("probe: %+v", info)
	}
	if info.NotAfter.IsZero() {
		t.Fatal("срок сертификата не прочитан")
	}
}
