package api

import (
	"net/http"
	"net/url"
	"strings"
)

// CORS для приложений (C123, решение C69): страница Android-клиента открыта
// с адреса одного сервера, а запросы к другому идут напрямую из WebView.
// Разрешаем только маршруты участника: панель, bootstrap и одноразовые
// ссылки на архивы остаются закрытыми. Вход по Bearer-токену, cookie нет,
// поэтому Access-Control-Allow-Credentials не выставляется.
const (
	corsAllowMethods  = "GET, POST, PUT, PATCH, DELETE, OPTIONS"
	corsAllowHeaders  = "Authorization, Content-Type, Range, Content-Range, Upload-Offset, If-Range, If-None-Match, If-Modified-Since"
	corsExposeHeaders = "Retry-After, Content-Range, Content-Disposition, ETag, Upload-Offset, Accept-Ranges, Location"
	corsMaxAge        = "600"
)

// serveParticipantCORS adds CORS headers to participant API responses and
// answers preflight. It reports whether the request was fully handled.
func serveParticipantCORS(w http.ResponseWriter, r *http.Request) bool {
	if !corsPath(r.URL.Path) {
		return false
	}
	// Ответ зависит от Origin: общий кеш не должен отдать ответ без
	// заголовков запросу с чужим Origin.
	w.Header().Add("Vary", "Origin")
	origin := r.Header.Get("Origin")
	if origin == "" || !corsOriginOK(origin) {
		return false
	}
	h := w.Header()
	h.Set("Access-Control-Allow-Origin", origin)
	h.Set("Access-Control-Expose-Headers", corsExposeHeaders)
	if r.Method != http.MethodOptions || r.Header.Get("Access-Control-Request-Method") == "" {
		return false
	}
	h.Set("Access-Control-Allow-Methods", corsAllowMethods)
	h.Set("Access-Control-Allow-Headers", corsAllowHeaders)
	h.Set("Access-Control-Max-Age", corsMaxAge)
	w.WriteHeader(http.StatusNoContent)
	return true
}

// corsPath reports whether the path belongs to the participant API. The admin
// panel, bootstrap and one-time archive links keep the browser's same-origin
// rule even though they live under /api/v1.
func corsPath(path string) bool {
	if !strings.HasPrefix(path, "/api/v1/") {
		return false
	}
	if path == "/api/v1/admin" || strings.HasPrefix(path, "/api/v1/admin/") {
		return false
	}
	if strings.HasPrefix(path, "/api/v1/archive/") ||
		strings.HasPrefix(path, "/api/v1/full_archive_stream/") {
		return false
	}
	return true
}

// corsOriginOK accepts only a bare http(s) origin: scheme://host[:port], with
// no path, query, fragment, userinfo or opaque part. Browsers send exactly
// that; anything else is not a browser origin and must not be echoed.
func corsOriginOK(origin string) bool {
	if len(origin) > 2048 {
		return false
	}
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" || u.User != nil || u.Opaque != "" {
		return false
	}
	if scheme := strings.ToLower(u.Scheme); scheme != "http" && scheme != "https" {
		return false
	}
	return u.Path == "" && u.RawQuery == "" && u.Fragment == ""
}
