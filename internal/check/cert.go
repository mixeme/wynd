package check

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"
)

// VerifyError — почему браузер не доверит сертификату (пусто — доверит).
type VerifyError string

const (
	VerifyExpired          VerifyError = "expired"
	VerifyHostname         VerifyError = "hostname"
	VerifySelfSigned       VerifyError = "self_signed"
	VerifyUnknownAuthority VerifyError = "unknown_authority"
	VerifyOther            VerifyError = "other"
)

// TLSInfo is filled by ProbeTLS for certificate checks.
type TLSInfo struct {
	NotBefore  time.Time
	NotAfter   time.Time
	Issuer     string
	IssuerOrg  string
	FullChain  bool
	Verify     VerifyError
	ProbeError string
}

// ProbeTLS fetches the public TLS certificate chain for publicURL (https://…).
//
// Сертификат проверяется как в браузере; при отказе проверки цепочка всё
// равно достаётся из tls.CertificateVerificationError — без
// InsecureSkipVerify, — и причина раскладывается по видам. Раньше любой
// отказ давал одно «не удалось прочитать сертификат», и строки «истёк» и
// «неполная цепочка» были недостижимы (план 42, CHK-2).
func ProbeTLS(ctx context.Context, publicURL string) *TLSInfo {
	ep, ok := parsePublicURL(publicURL)
	if !ok || !ep.HTTPS {
		return &TLSInfo{ProbeError: "нет HTTPS public_url"}
	}
	dialer := &tls.Dialer{
		NetDialer: &net.Dialer{Timeout: 10 * time.Second},
		Config:    &tls.Config{ServerName: ep.Host},
	}
	conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(ep.Host, ep.Port))
	if err != nil {
		return classifyTLSError(err)
	}
	defer conn.Close()
	state := conn.(*tls.Conn).ConnectionState()
	if len(state.PeerCertificates) == 0 {
		return &TLSInfo{ProbeError: "нет сертификата"}
	}
	info := certInfo(state.PeerCertificates[0])
	info.FullChain = len(state.PeerCertificates) > 1
	if !info.FullChain {
		// Сервер прислал только лист, а рукопожатие прошло: либо лист подписан
		// прямо корнем (цепочка и правда полная), либо системный проверщик сам
		// докачал промежуточный (Windows делает это по AIA) — у старых
		// клиентов его не будет.
		roots, _ := x509.SystemCertPool()
		info.FullChain = roots != nil && isRoot(state.PeerCertificates[0], roots)
	}
	return info
}

func certInfo(leaf *x509.Certificate) *TLSInfo {
	return &TLSInfo{
		NotBefore: leaf.NotBefore.UTC(),
		NotAfter:  leaf.NotAfter.UTC(),
		Issuer:    leaf.Issuer.CommonName,
		IssuerOrg: strings.Join(leaf.Issuer.Organization, " "),
	}
}

// isRoot — лист подписан прямо корнем из хранилища: промежуточного нет вовсе.
func isRoot(leaf *x509.Certificate, roots *x509.CertPool) bool {
	chains, err := leaf.Verify(x509.VerifyOptions{Roots: roots})
	return err == nil && len(chains) > 0 && len(chains[0]) == 2
}

func classifyTLSError(err error) *TLSInfo {
	var cve *tls.CertificateVerificationError
	if !errors.As(err, &cve) || len(cve.UnverifiedCertificates) == 0 {
		return &TLSInfo{ProbeError: err.Error()}
	}
	leaf := cve.UnverifiedCertificates[0]
	info := certInfo(leaf)
	var invalid x509.CertificateInvalidError
	var hostname x509.HostnameError
	var unknown x509.UnknownAuthorityError
	switch {
	case errors.As(err, &invalid) && invalid.Reason == x509.Expired:
		info.Verify = VerifyExpired
	case errors.As(err, &hostname):
		info.Verify = VerifyHostname
	case errors.As(err, &unknown):
		if len(cve.UnverifiedCertificates) == 1 && bytes.Equal(leaf.RawIssuer, leaf.RawSubject) {
			info.Verify = VerifySelfSigned
		} else {
			info.Verify = VerifyUnknownAuthority
		}
	default:
		info.Verify = VerifyOther
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
	switch in.TLS.Verify {
	case VerifyHostname:
		r.Status = StatusFail
		r.Detail = "выпущен не для этого домена"
		return r
	case VerifySelfSigned:
		r.Status = StatusFail
		r.Detail = "самоподписанный — браузеры не доверяют"
		return r
	}
	remaining := in.TLS.NotAfter.Sub(in.Now.UTC())
	days := int(remaining.Hours() / 24)
	if remaining < 0 || in.TLS.Verify == VerifyExpired {
		r.Status = StatusFail
		r.Detail = fmt.Sprintf("истёк %s", formatCertDate(in.TLS.NotAfter))
		return r
	}
	r.Status = StatusOK
	if remaining < renewalOverdue(in.TLS) {
		r.Status = StatusWarn
	}
	if isLetsEncryptIssuer(in.TLS) {
		r.Detail = fmt.Sprintf("до %s, %d дн.", formatCertDate(in.TLS.NotAfter), days)
		return r
	}
	issuer := in.TLS.IssuerOrg
	if issuer == "" {
		issuer = in.TLS.Issuer
	}
	r.Detail = fmt.Sprintf("выпустил %s; до %s, %d дн.", issuer, formatCertDate(in.TLS.NotAfter), days)
	return r
}

// renewalOverdue — сколько срока должно остаться у исправно продлеваемого
// сертификата. ACME-клиенты (certbot, Caddy) продлевают за треть срока;
// четверть — продление уже должно было пройти. Доля, а не 30 дней: Let’s
// Encrypt переходит на 45-дневные сертификаты, и фиксированный порог горел
// бы у исправного инстанса постоянно (CHK-3). Без NotBefore — 90 дней.
func renewalOverdue(info *TLSInfo) time.Duration {
	lifetime := 90 * 24 * time.Hour
	if !info.NotBefore.IsZero() && info.NotAfter.After(info.NotBefore) {
		lifetime = info.NotAfter.Sub(info.NotBefore)
	}
	return lifetime / 4
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
	switch in.TLS.Verify {
	case VerifyUnknownAuthority:
		r.Status = StatusFail
		r.Detail = "не проверяется: неполная или выпущена неизвестным центром"
		return r
	case VerifySelfSigned:
		r.Status = StatusFail
		r.Detail = "нет — сертификат самоподписанный"
		return r
	case VerifyExpired, VerifyHostname, VerifyOther:
		r.Status = StatusWarn
		r.Detail = "не проверить: сам сертификат недействителен"
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

// isLetsEncryptIssuer смотрит только на организацию издателя. Догадка по CN
// вида «R10»/«E5» совпадала с любым чужим центром с таким именем (CHK-3).
func isLetsEncryptIssuer(info *TLSInfo) bool {
	org := strings.ToLower(info.IssuerOrg)
	return strings.Contains(org, "let's encrypt") || strings.Contains(org, "let’s encrypt") || strings.Contains(org, "lets encrypt")
}

func formatCertDate(t time.Time) string {
	months := []string{
		"января", "февраля", "марта", "апреля", "мая", "июня",
		"июля", "августа", "сентября", "октября", "ноября", "декабря",
	}
	d := t.UTC()
	return fmt.Sprintf("%d %s", d.Day(), months[d.Month()-1])
}
