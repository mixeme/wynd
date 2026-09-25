package web

import (
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// SPA serves static files from root and falls back to index.html for client routes.
func SPA(root fs.FS) http.Handler {
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
				setSPAAssetCachePolicy(w.Header(), name)
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

// setSPAAssetCachePolicy keeps hashed bundles cacheable but forces revalidation
// of the shell and service worker so local rebuilds show up without clearing
// site data.
func setSPAAssetCachePolicy(h http.Header, name string) {
	base := path.Base(name)
	switch {
	case base == "sw.js", strings.HasPrefix(base, "workbox-"), base == "index.html":
		h.Set("Cache-Control", "no-cache")
	case strings.HasSuffix(name, ".webmanifest"):
		h.Set("Cache-Control", "no-cache")
	case strings.HasPrefix(name, "_app/"):
		// Hashed filenames usually bust caches, but during local rebuilds the
		// hash can stay stable while content changes — force revalidation.
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
