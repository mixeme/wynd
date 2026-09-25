package api

import (
	"encoding/json"
	"errors"
	"log"
	"net"
	"net/http"
	"strings"

	"gitea.mixdep.ru/mix/wynd/internal/auth"
)

// maxJSONBody caps request bodies on JSON endpoints. Attachments never travel
// through JSON (they go through the chunked upload routes, bounded by the
// admin attachment limit), so 1 MiB covers any real post or settings payload.
const maxJSONBody = 1 << 20

var errBodyTooLarge = errors.New("api: body too large")

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("api: json encode: %v", err)
	}
}

func writeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, errBodyTooLarge):
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{"error": "payload_too_large"})
	case errors.Is(err, auth.ErrWeakPassword):
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "weak_password"})
	case errors.Is(err, auth.ErrInvalid):
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid"})
	case errors.Is(err, auth.ErrNotFound):
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not_found"})
	case errors.Is(err, auth.ErrForbidden):
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
	case errors.Is(err, auth.ErrConflict):
		writeJSON(w, http.StatusConflict, map[string]string{"error": "conflict"})
	case errors.Is(err, auth.ErrRateLimited):
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "rate_limited"})
	case errors.Is(err, auth.ErrTooManyAttempts):
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "too_many_attempts"})
	case errors.Is(err, auth.ErrExpired):
		writeJSON(w, http.StatusGone, map[string]string{"error": "expired"})
	case errors.Is(err, auth.ErrClosed):
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "registration_closed"})
	default:
		writeDomainError(w, err)
	}
}

// readJSON decodes a JSON body. Bodies over maxJSONBody (enforced by
// limitBody) surface as errBodyTooLarge → 413; anything else is invalid.
func readJSON(r *http.Request, v any) error {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		var mbe *http.MaxBytesError
		if errors.As(err, &mbe) {
			return errBodyTooLarge
		}
		return auth.ErrInvalid
	}
	return nil
}

// limitBody wraps a handler so its request body is capped at maxJSONBody.
// Do not use it on the upload chunk route.
func limitBody(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Body != nil {
			r.Body = http.MaxBytesReader(w, r.Body, maxJSONBody)
		}
		next(w, r)
	}
}

// ParseTrustedProxies turns CIDRs (or bare IPs) into networks. An empty list
// means "trust loopback only".
func ParseTrustedProxies(entries []string) ([]*net.IPNet, error) {
	var out []*net.IPNet
	for _, raw := range entries {
		e := strings.TrimSpace(raw)
		if e == "" {
			continue
		}
		if !strings.Contains(e, "/") {
			ip := net.ParseIP(e)
			if ip == nil {
				return nil, errors.New("trusted proxy: bad address " + e)
			}
			bits := 32
			if ip.To4() == nil {
				bits = 128
			}
			e = ip.String() + "/" + itoa(bits)
		}
		_, n, err := net.ParseCIDR(e)
		if err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, nil
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [4]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

func (s *Server) trustedProxy(ip string) bool {
	parsed := net.ParseIP(strings.Trim(ip, "[]"))
	if parsed == nil {
		return false
	}
	if len(s.TrustedProxies) == 0 {
		return parsed.IsLoopback()
	}
	for _, n := range s.TrustedProxies {
		if n.Contains(parsed) {
			return true
		}
	}
	return false
}

// clientIP returns the caller address for rate limiting. X-Forwarded-For is
// honoured only when the direct peer is a trusted proxy; the rightmost
// address not belonging to a trusted proxy wins, so a client cannot pick its
// own bucket by sending the header.
func (s *Server) clientIP(r *http.Request) string {
	remote := remoteHost(r.RemoteAddr)
	if !s.trustedProxy(remote) {
		return remote
	}
	xff := r.Header.Get("X-Forwarded-For")
	if xff == "" {
		return remote
	}
	parts := strings.Split(xff, ",")
	for i := len(parts) - 1; i >= 0; i-- {
		ip := strings.TrimSpace(parts[i])
		if ip == "" {
			continue
		}
		if !s.trustedProxy(ip) {
			return ip
		}
	}
	if first := strings.TrimSpace(parts[0]); first != "" {
		return first
	}
	return remote
}

func remoteHost(addr string) string {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	return host
}

func bearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if len(h) < len(prefix) || h[:len(prefix)] != prefix {
		return ""
	}
	return h[len(prefix):]
}
