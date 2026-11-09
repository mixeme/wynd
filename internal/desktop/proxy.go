package desktop

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"sync"
)

// Proxy отдаёт серверы через один origin окна: страница выбранного сервера
// живёт в iframe, её запросы к /api/v1, потоку событий и медиа идут сюда.
// Снаружи можно попасть только на серверы, которые уже добавлены: адрес
// берётся из хранилища, а не из пути запроса.
type Proxy struct {
	mu      sync.RWMutex
	target  *url.URL
	rp      *httputil.ReverseProxy
	allowed map[string]*url.URL
	foreign map[string]*httputil.ReverseProxy
}

// NewProxy — прокси без выбранного сервера: до выбора отдаёт 503.
func NewProxy() *Proxy {
	return &Proxy{
		allowed: make(map[string]*url.URL),
		foreign: make(map[string]*httputil.ReverseProxy),
	}
}

func newReverseProxy(u *url.URL, injectBridge bool) *httputil.ReverseProxy {
	rp := httputil.NewSingleHostReverseProxy(u)
	base := rp.Director
	rp.Director = func(req *http.Request) {
		base(req)
		req.Host = u.Host
		// Origin браузера относится к окну, а не к серверу; сервер не должен
		// решать, что запрос пришёл с чужого адреса.
		req.Header.Del("Origin")
		// HTML правим ниже, поэтому сжатый ответ не годится.
		req.Header.Del("Accept-Encoding")
	}
	// Поток событий не должен ждать наполнения буфера.
	rp.FlushInterval = -1
	// Сервер запрещает встраивание в чужие страницы (X-Frame-Options: DENY,
	// frame-ancestors 'none'). Окно и iframe — один origin, поэтому тому же
	// origin встраивание разрешаем; чужим — по-прежнему нет.
	//
	// В HTML выбранного сервера добавляем мост: страница видит свой настоящий
	// адрес, хотя окно живёт на wails.localhost. Для чужих серверов (/__srv)
	// мост не нужен: там только запросы, а не страница.
	marker := []byte(`<script>window.__wyndDesktop={"origin":` + strconv.Quote(u.String()) + `};</script>`)
	rp.ModifyResponse = func(res *http.Response) error {
		res.Header.Del("X-Frame-Options")
		if values := res.Header.Values("Content-Security-Policy"); len(values) > 0 {
			rewritten := make([]string, 0, len(values))
			for _, value := range values {
				rewritten = append(rewritten, strings.ReplaceAll(value, "frame-ancestors 'none'", "frame-ancestors 'self'"))
			}
			res.Header["Content-Security-Policy"] = rewritten
		}
		if !injectBridge || !strings.HasPrefix(res.Header.Get("Content-Type"), "text/html") ||
			res.Header.Get("Content-Encoding") != "" {
			return nil
		}
		body, err := io.ReadAll(res.Body)
		if err != nil {
			return err
		}
		_ = res.Body.Close()
		if !bytes.Contains(body, []byte("__wyndDesktop")) {
			if at := bytes.Index(bytes.ToLower(body), []byte("</head>")); at >= 0 {
				next := make([]byte, 0, len(body)+len(marker))
				next = append(next, body[:at]...)
				next = append(next, marker...)
				next = append(next, body[at:]...)
				body = next
			} else {
				body = append(marker, body...)
			}
		}
		res.Body = io.NopCloser(bytes.NewReader(body))
		res.ContentLength = int64(len(body))
		res.Header.Set("Content-Length", strconv.Itoa(len(body)))
		return nil
	}
	rp.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, err error) {
		http.Error(w, "Сервер не отвечает", http.StatusBadGateway)
	}
	return rp
}

// Set переключает сервер. Пустой адрес снимает выбор.
func (p *Proxy) Set(origin string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if origin == "" {
		p.target, p.rp = nil, nil
		return nil
	}
	u, err := url.Parse(origin)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return fmt.Errorf("%w: %s", ErrInvalidAddress, origin)
	}
	p.target, p.rp = u, newReverseProxy(u, true)
	return nil
}

// SetAllowed задаёт серверы, к которым окно может обращаться через /__srv.
// Список приходит из хранилища; адрес из запроса сам по себе доступа не даёт.
func (p *Proxy) SetAllowed(origins []string) error {
	next := make(map[string]*url.URL, len(origins))
	for _, origin := range origins {
		u, err := url.Parse(origin)
		if err != nil || u.Scheme == "" || u.Host == "" {
			return fmt.Errorf("%w: %s", ErrInvalidAddress, origin)
		}
		next[u.Scheme+"://"+u.Host] = u
	}
	p.mu.Lock()
	p.allowed = next
	p.foreign = make(map[string]*httputil.ReverseProxy, len(next))
	p.mu.Unlock()
	return nil
}

// Target — выбранный адрес для проверок и смены сервера.
func (p *Proxy) Target() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.target == nil {
		return ""
	}
	return p.target.String()
}

// Handler раскладывает запросы окна:
//   - / и /__shell/* — оболочка (index.html и её файлы);
//   - /__server/* и /?wynd=server — начальная страница сервера в iframe;
//   - /__srv/<scheme>/<host>/… — другой добавленный сервер (API, события, медиа);
//   - остальное — выбранный сервер: API, события, медиа, переходы.
//
// Корень отдаёт оболочку. Iframe просит его с меткой ?wynd=server: так адрес
// страницы остаётся /, и клиент сервера видит свой обычный маршрут, а не
// служебный путь оболочки. Метка снимается до сервера.
func (p *Proxy) Handler(shell http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		switch {
		case path == "/" || path == "/index.html":
			if r.URL.Query().Get("wynd") == "server" || r.Header.Get("Sec-Fetch-Dest") == "iframe" {
				p.serve(w, r, "/")
				return
			}
			shell.ServeHTTP(w, r)
		case strings.HasPrefix(path, "/__shell"):
			rest := strings.TrimPrefix(path, "/__shell")
			if rest == "" {
				rest = "/"
			}
			r2 := r.Clone(r.Context())
			r2.URL.Path = rest
			r2.URL.RawPath = ""
			shell.ServeHTTP(w, r2)
		case strings.HasPrefix(path, "/__server"):
			rest := strings.TrimPrefix(path, "/__server")
			if rest == "" {
				rest = "/"
			}
			p.serve(w, r, rest)
		case strings.HasPrefix(path, "/__srv/"):
			p.serveForeign(w, r)
		default:
			p.serve(w, r, path)
		}
	})
}

func (p *Proxy) serve(w http.ResponseWriter, r *http.Request, path string) {
	p.mu.RLock()
	rp := p.rp
	p.mu.RUnlock()
	if rp == nil {
		http.Error(w, "Сервер не выбран", http.StatusServiceUnavailable)
		return
	}
	p.serveWith(rp, w, r, path)
}

// serveForeign — /__srv/<scheme>/<host>/<path> для добавленного сервера.
func (p *Proxy) serveForeign(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/__srv/")
	scheme, rest, ok := strings.Cut(rest, "/")
	if !ok || (scheme != "http" && scheme != "https") {
		http.NotFound(w, r)
		return
	}
	host, tail, ok := strings.Cut(rest, "/")
	if host == "" {
		http.NotFound(w, r)
		return
	}
	if !ok {
		tail = ""
	}
	key := scheme + "://" + host
	p.mu.Lock()
	target, allowed := p.allowed[key]
	if !allowed {
		p.mu.Unlock()
		http.Error(w, "Сервер не добавлен", http.StatusForbidden)
		return
	}
	rp := p.foreign[key]
	if rp == nil {
		rp = newReverseProxy(target, false)
		p.foreign[key] = rp
	}
	p.mu.Unlock()
	p.serveWith(rp, w, r, "/"+tail)
}

func (p *Proxy) serveWith(rp *httputil.ReverseProxy, w http.ResponseWriter, r *http.Request, path string) {
	r2 := r.Clone(r.Context())
	r2.URL.Path = path
	r2.URL.RawPath = ""
	if q := r2.URL.Query(); q.Get("wynd") == "server" {
		q.Del("wynd")
		r2.URL.RawQuery = q.Encode()
	}
	rp.ServeHTTP(w, r2)
}
