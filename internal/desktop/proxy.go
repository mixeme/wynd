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
	// apiBase — адрес локального HTTP-порта окна: страница берёт отсюда API и
	// поток событий. Через WebView2 ответы буферизуются (SSE не течёт),
	// поэтому запросы клиента идут по обычному TCP на 127.0.0.1.
	apiBase string
	// embedded — вшитый клиент (план 66, функция 9): страницы клиента из exe.
	// Источник интерфейса — настройка окна (source): вшитый по умолчанию,
	// серверный — явный откат.
	embedded http.Handler
	// source — текущий источник интерфейса: SourceEmbedded или SourceServer.
	// По умолчанию (поле пустое) — серверный, как у прокси без вшитого клиента.
	source string
}

// NewProxy — прокси без выбранного сервера: до выбора отдаёт 503.
func NewProxy() *Proxy {
	return &Proxy{
		allowed: make(map[string]*url.URL),
		foreign: make(map[string]*httputil.ReverseProxy),
	}
}

// SetAPIBase задаёт адрес локального порта до первого Set: он попадает в
// мост страницы как "apiBase".
func (p *Proxy) SetAPIBase(base string) {
	p.mu.Lock()
	p.apiBase = base
	p.mu.Unlock()
}

func (p *Proxy) bridgeAPIBase() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.apiBase
}

func newReverseProxy(u *url.URL, injectBridge bool, apiBase string) *httputil.ReverseProxy {
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
	marker := []byte(`<script>window.__wyndDesktop={"origin":` + strconv.Quote(u.String()) +
		`,"apiBase":` + strconv.Quote(apiBase) + `};</script>`)
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
		if !bytes.Contains(body, bridgeMarker) {
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
	// Сервер недоступен — рвём соединение, как будто сети нет. Ответ 502
	// страница принимала за ответ API (isTransportError), и очередь отправки
	// не ставила записи в ожидание; в браузере без сервера запрос падает
	// сетевым сбоем, и окно должно вести себя так же (C66, «дозагрузка»).
	rp.ErrorHandler = func(w http.ResponseWriter, r *http.Request, _ error) {
		// Клиент уже ушёл: переход или перезагрузка отменили ответ, рвать
		// нечего. Паника здесь роняла окно целиком: asset-сервер Wails v3
		// не перехватывает http.ErrAbortHandler (переход по ссылке
		// приглашения сразу после запуска, найдено 27.11 на Linux).
		if r.Context().Err() != nil {
			return
		}
		if hj, ok := w.(http.Hijacker); ok {
			if conn, _, err := hj.Hijack(); err == nil {
				_ = conn.Close()
				return
			}
		}
		// Asset-сервер Wails соединение не отдаёт (Hijack нет). Обрыв сети
		// изображаем недоданным телом: WebKit/WebView2 не досчитают
		// Content-Length, fetch упадёт сетевым сбоем, как без сервера.
		w.Header().Set("Content-Length", "1")
		w.WriteHeader(http.StatusOK)
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
	p.target, p.rp = u, newReverseProxy(u, true, p.apiBase)
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

// SetEmbeddedUI включает отдачу вшитого клиента вместо страниц сервера.
// Серверные пути (/api/, /archive/, установка) по-прежнему идут в выбранный
// сервер; оболочка и /__srv не меняются. Какой источник показывать, решает
// SetSource.
func (p *Proxy) SetEmbeddedUI(handler http.Handler) {
	p.mu.Lock()
	p.embedded = handler
	p.mu.Unlock()
}

// SetSource переключает источник интерфейса на ходу: вшитый клиент из exe или
// страницы выбранного сервера. Неизвестное значение — серверный режим.
func (p *Proxy) SetSource(source string) {
	p.mu.Lock()
	if source == SourceEmbedded {
		p.source = SourceEmbedded
	} else {
		p.source = SourceServer
	}
	p.mu.Unlock()
}

func (p *Proxy) sourceMode() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.source == SourceEmbedded {
		return SourceEmbedded
	}
	return SourceServer
}

// embeddedHandler — текущий вшитый клиент или nil.
func (p *Proxy) embeddedHandler() http.Handler {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.embedded
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
//   - /?wynd=embedded — вшитый клиент в iframe;
//   - /__srv/<scheme>/<host>/… — другой добавленный сервер (API, события, медиа);
//   - остальное — выбранный сервер: API, события, медиа, переходы.
//
// Корень отдаёт оболочку; iframe просит источник меткой ?wynd=server или
// ?wynd=embedded, чтобы адрес страницы остался / и клиент видел свой обычный
// маршрут, а не служебный путь оболочки. Метка снимается до страницы. Без
// метки iframe получает текущий источник окна: переходы внутри клиента на /
// не должны незаметно менять вшитый клиент на серверный.
func (p *Proxy) Handler(shell http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		marker := r.URL.Query().Get("wynd")
		if embedded := p.embeddedHandler(); embedded != nil && embeddedPath(path) {
			// Вшитый клиент (план 66, функция 9): страницы — из exe, данные —
			// с выбранного сервера. Служебные пути окна остаются за прокси.
			switch {
			case path == "/" && marker == "server":
				p.serve(w, r, "/")
				return
			case path == "/" && marker == "embedded":
				embedded.ServeHTTP(w, withoutWynd(r))
				return
			case (path == "/" || path == "/index.html") && !isIframe(r):
				shell.ServeHTTP(w, r)
				return
			case p.sourceMode() == SourceEmbedded:
				if serverOnlyPath(path) {
					p.serve(w, r, path)
				} else {
					embedded.ServeHTTP(w, withoutWynd(r))
				}
				return
			}
			// Серверный режим — обычная раскладка ниже, как без вшитого клиента.
		}
		switch {
		case path == "/" || path == "/index.html":
			if marker == "server" || isIframe(r) {
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

// isIframe — запрос главной страницы пришёл из рамки интерфейса.
func isIframe(r *http.Request) bool {
	return r.Header.Get("Sec-Fetch-Dest") == "iframe"
}

// withoutWynd снимает служебную метку источника с адреса страницы: клиент не
// должен видеть ?wynd=… в своём маршруте.
func withoutWynd(r *http.Request) *http.Request {
	if r.URL.Query().Get("wynd") == "" {
		return r
	}
	r2 := r.Clone(r.Context())
	q := r2.URL.Query()
	q.Del("wynd")
	r2.URL.RawQuery = q.Encode()
	return r2
}

// embeddedPath — пути вшитого клиента против служебных путей окна.
func embeddedPath(path string) bool {
	switch {
	case path == "/__shell" || strings.HasPrefix(path, "/__shell/"):
		return false
	case path == "/__server" || strings.HasPrefix(path, "/__server/"):
		return false
	case strings.HasPrefix(path, "/__srv/"):
		return false
	}
	return true
}

// serverOnlyPath — то, что во вшитом режиме остаётся страницами сервера:
// API и поток событий, одноразовые ссылки архива, установка сервера.
func serverOnlyPath(path string) bool {
	switch {
	case path == "/api" || strings.HasPrefix(path, "/api/"):
		return true
	case path == "/archive" || strings.HasPrefix(path, "/archive/"):
		return true
	case path == "/admin/bootstrap" || strings.HasPrefix(path, "/admin/bootstrap/"):
		return true
	}
	return false
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
		rp = newReverseProxy(target, false, p.apiBase)
		p.foreign[key] = rp
	}
	p.mu.Unlock()
	p.serveWith(rp, w, r, "/"+tail)
}

func (p *Proxy) serveWith(rp *httputil.ReverseProxy, w http.ResponseWriter, r *http.Request, path string) {
	r2 := r.Clone(r.Context())
	r2.URL.Path = path
	r2.URL.RawPath = ""
	rp.ServeHTTP(w, withoutWynd(r2))
}
