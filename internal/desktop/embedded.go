package desktop

import (
	"bytes"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strconv"
	"strings"
)

// Вшитый клиент (план 66, функция 9). Клиент отдаётся из exe теми же путями,
// что раньше отдавал сервер, поэтому маршруты, ссылки и IndexedDB не меняются:
// origin окна прежний, переносить данные не нужно. В страницу вставляется тот
// же мост, что и в HTML сервера, — клиент видит настоящий адрес сервера и
// локальный порт API. Включается флагом --embedded-ui; переключение отложено
// до стабилизации интерфейса (решение владельца 23.11).

// EmbeddedHandler отдаёт файлы вшитого клиента с SPA-фолбэком на index.html.
// origin — выбранный сервер для моста (может быть пустым: моста тогда нет),
// apiBase — локальный порт API окна.
func EmbeddedHandler(dist fs.FS, origin func() string, apiBase string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		name := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
		if name == "" {
			name = "index.html"
		}
		data, err := fs.ReadFile(dist, name)
		isIndex := name == "index.html"
		if err != nil {
			// Расширение — это файл; без расширения — маршрут клиента (SPA).
			if strings.Contains(path.Base(name), ".") {
				http.NotFound(w, r)
				return
			}
			if data, err = fs.ReadFile(dist, "index.html"); err != nil {
				http.Error(w, "клиент не вшит", http.StatusInternalServerError)
				return
			}
			name = "index.html"
			isIndex = true
		}
		if isIndex {
			data = injectBridge(data, origin(), apiBase)
			w.Header().Set("Cache-Control", "no-cache")
		} else {
			w.Header().Set("Cache-Control", "public, max-age=3600")
		}
		if ctype := mime.TypeByExtension(path.Ext(name)); ctype != "" {
			w.Header().Set("Content-Type", ctype)
		}
		w.Header().Set("Content-Length", strconv.Itoa(len(data)))
		w.WriteHeader(http.StatusOK)
		if r.Method == http.MethodHead {
			return
		}
		_, _ = io.Copy(w, bytes.NewReader(data))
	})
}

// injectBridge вставляет мост окна в index.html до </head>; уже вставленный
// мост (страница из старой сборки) не дублируется.
func injectBridge(body []byte, origin, apiBase string) []byte {
	if bytes.Contains(body, []byte("__wyndDesktop")) {
		return body
	}
	marker := []byte(`<script>window.__wyndDesktop={"origin":` + strconv.Quote(origin) +
		`,"apiBase":` + strconv.Quote(apiBase) + `};</script>`)
	if at := bytes.Index(bytes.ToLower(body), []byte("</head>")); at >= 0 {
		next := make([]byte, 0, len(body)+len(marker))
		next = append(next, body[:at]...)
		next = append(next, marker...)
		next = append(next, body[at:]...)
		return next
	}
	return append(marker, body...)
}
