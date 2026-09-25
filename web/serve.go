package web

import (
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// SPA serves static files from root and falls back to index.html for client routes.
// loopback — локальная разработка: хешированные чанки тогда тоже ревалидируются.
func SPA(root fs.FS, loopback bool) http.Handler {
	files := http.FileServer(http.FS(root))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.NotFound(w, r)
			return
		}
		setSecurityHeaders(w.Header())

		name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if name == "." {
			name = ""
		}
		if name != "" {
			// Каталог (/_app/, /fonts/) — 404, иначе FileServer отдаёт листинг (API-6).
			if info, err := fs.Stat(root, name); err == nil && info.IsDir() {
				http.NotFound(w, r)
				return
			} else if err == nil {
				// Go's MIME table has no .webmanifest, and the sniffed text/plain
				// makes browsers ignore the manifest.
				if strings.HasSuffix(name, ".webmanifest") {
					w.Header().Set("Content-Type", "application/manifest+json")
				}
				setSPAAssetCachePolicy(w.Header(), name, loopback)
				files.ServeHTTP(w, r)
				return
			}
			if strings.Contains(path.Base(name), ".") {
				http.NotFound(w, r)
				return
			}
		}

		r2 := r.Clone(r.Context())
		r2.URL.Path = "/"
		w.Header().Set("Cache-Control", "no-cache")
		files.ServeHTTP(w, r2)
	})
}

// immutableCacheControl — год: имя файла в `_app/immutable/` меняется вместе
// с содержимым, так что перепроверять его незачем.
const immutableCacheControl = "public, max-age=31536000, immutable"

// setSPAAssetCachePolicy forces revalidation of the shell, service worker and
// unhashed files; hashed bundles under `_app/immutable/` are cached for a year
// except on loopback.
//
// Раньше `no-cache` стоял на всём `_app/` и в продакшене: каждый старт
// приложения без service worker'а перепроверял все чанки (план 42, SW-2).
// На loopback при локальной пересборке хеш бывает прежним при новом
// содержимом — там ревалидация остаётся.
func setSPAAssetCachePolicy(h http.Header, name string, loopback bool) {
	base := path.Base(name)
	switch {
	case base == "sw.js", strings.HasPrefix(base, "workbox-"), base == "index.html":
		h.Set("Cache-Control", "no-cache")
	case strings.HasSuffix(name, ".webmanifest"):
		h.Set("Cache-Control", "no-cache")
	case strings.HasPrefix(name, "_app/immutable/") && !loopback:
		h.Set("Cache-Control", immutableCacheControl)
	case strings.HasPrefix(name, "_app/"):
		h.Set("Cache-Control", "no-cache")
	}
}

// setSecurityHeaders adds browser hardening that does not depend on the
// reverse proxy. The CSP is deliberately narrow: it forbids framing, plugins
// and <base> hijacking without constraining scripts, styles or the tile and
// API origins the client talks to. HSTS belongs on the TLS terminator.
func setSecurityHeaders(h http.Header) {
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("X-Frame-Options", "DENY")
	h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
	h.Set("Content-Security-Policy", "frame-ancestors 'none'; object-src 'none'; base-uri 'self'")
}
