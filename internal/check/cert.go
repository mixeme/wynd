package check

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"strings"
	"time"
)

// TLSInfo is filled by ProbeTLS for certificate checks.
type TLSInfo struct {
	NotAfter   time.Time
	Issuer     string
	IssuerOrg  string
	FullChain  bool
	ProbeError string
}

// ProbeTLS fetches the public TLS certificate chain for publicURL (https://…).
func ProbeTLS(ctx context.Context, publicURL string) *TLSInfo {
	host := publicHost(publicURL)
	if host == "" || !strings.HasPrefix(strings.ToLower(publicURL), "https://") {
		return &TLSInfo{ProbeError: "нет HTTPS public_url"}
	}
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	conn, err := tls.DialWithDialer(dialer, "tcp", host+":443", &tls.Config{
		ServerName: host,
	})
	if err != nil {
		return &TLSInfo{ProbeError: err.Error()}
	}
	defer conn.Close()
	state := conn.ConnectionState()
	if len(state.PeerCertificates) == 0 {
		return &TLSInfo{ProbeError: "нет сертификата"}
	}
	leaf := state.PeerCertificates[0]
	info := &TLSInfo{
		NotAfter:  leaf.NotAfter.UTC(),
		Issuer:    leaf.Issuer.CommonName,
		IssuerOrg: strings.Join(leaf.Issuer.Organization, " "),
		FullChain: len(state.PeerCertificates) > 1,
	}
	if !info.FullChain {
		roots, _ := x509.SystemCertPool()
		if roots != nil {
			opts := x509.VerifyOptions{
				DNSName: host,
				Roots:   roots,
				Intermediates: x509.NewCertPool(),
			}
			for _, c := range state.PeerCertificates[1:] {
				opts.Intermediates.AddCert(c)
			}
			if _, err := leaf.Verify(opts); err == nil {
				info.FullChain = true
			}
		}
	}
	return info
}

func checkCertLE(in Input) Result {
	r := Result{ID: "cert_le", Title: "Let’s Encrypt"}
	if in.Loopback {
		r.Status = StatusNA
		r.Detail = "на loopback не применимо"
		return r
	}
	if in.TLS == nil || in.TLS.ProbeError != "" {
		r.Status = StatusWarn
		if in.TLS != nil && in.TLS.ProbeError != "" {
			r.Detail = "не удалось прочитать сертификат"
		} else {
			r.Detail = "ожидается HTTPS public_url"
		}
		return r
	}
	days := int(in.TLS.NotAfter.Sub(in.Now.UTC()).Hours() / 24)
	if days < 0 {
		r.Status = StatusFail
		r.Detail = fmt.Sprintf("истёк %s", formatCertDate(in.TLS.NotAfter))
		return r
	}
	if isLetsEncryptIssuer(in.TLS) {
		r.Status = StatusOK
		if days <= 30 {
			r.Status = StatusWarn
		}
		r.Detail = fmt.Sprintf("до %s, %d дн.", formatCertDate(in.TLS.NotAfter), days)
		return r
	}
	r.Status = StatusWarn
	r.Detail = fmt.Sprintf("выпустил %s; до %s", in.TLS.Issuer, formatCertDate(in.TLS.NotAfter))
	return r
}

func checkCertChain(in Input) Result {
	r := Result{ID: "cert_chain", Title: "Цепочка"}
	if in.Loopback {
		r.Status = StatusNA
		r.Detail = "на loopback не применимо"
		return r
	}
	if in.TLS == nil || in.TLS.ProbeError != "" {
		r.Status = StatusWarn
		r.Detail = "не удалось проверить цепочку"
		return r
	}
	if in.TLS.FullChain {
		r.Status = StatusOK
		r.Detail = "полная — откроют и старые Android"
		return r
	}
	r.Status = StatusWarn
	r.Detail = "неполная — старые клиенты могут не доверять"
	return r
}

func isLetsEncryptIssuer(info *TLSInfo) bool {
	blob := strings.ToLower(info.Issuer + " " + info.IssuerOrg)
	if strings.Contains(blob, "let's encrypt") || strings.Contains(blob, "lets encrypt") {
		return true
	}
	cn := strings.ToLower(strings.TrimSpace(info.Issuer))
	if len(cn) < 2 || (cn[0] != 'r' && cn[0] != 'e') {
		return false
	}
	for _, c := range cn[1:] {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func formatCertDate(t time.Time) string {
	months := []string{
		"января", "февраля", "марта", "апреля", "мая", "июня",
		"июля", "августа", "сентября", "октября", "ноября", "декабря",
	}
	d := t.UTC()
	return fmt.Sprintf("%d %s", d.Day(), months[d.Month()-1])
}
