package check

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/proxy"
)

// Status is the outcome of a single instance check.
type Status string

const (
	StatusOK   Status = "ok"
	StatusWarn Status = "warn"
	StatusFail Status = "fail"
	StatusNA   Status = "na"
)

// ExternalReport is supplied by the admin browser running outside the server network.
type ExternalReport struct {
	HTTPSOK           bool   `json:"https_ok"`
	HTTPSMS           int    `json:"https_ms"`
	RedirectPermanent bool   `json:"redirect_permanent"`
	RedirectStatus    int    `json:"redirect_status,omitempty"`
	FromOutside       bool   `json:"from_outside"`
	ProxyHTTPS        bool   `json:"proxy_https"`
	ProxyBodyLimitOK  bool   `json:"proxy_body_limit_ok"`
	ProxySSEOK        bool   `json:"proxy_sse_ok"`
	ProxyStreamOK     bool   `json:"proxy_stream_ok"`
	ProxyReadTimeout  int    `json:"proxy_read_timeout_sec,omitempty"`
	PWAOK             bool   `json:"pwa_ok"`
	ClientIP          string `json:"client_ip"`
	XForwardedFor     string `json:"x_forwarded_for"`
	XRealIP           string `json:"x_real_ip"`
	// ProbeInstance — метка процесса из GET /probe, PageHost — имя, под
	// которым браузер открыл панель.
	ProbeInstance string `json:"probe_instance,omitempty"`
	PageHost      string `json:"page_host,omitempty"`
}

// Input drives RunChecks.
type Input struct {
	Loopback        bool
	PublicURL       string
	DataDir         string
	ServerEgressIP  string
	SMTPTestSentAt  *time.Time
	SMTPLastError   string
	VAPIDConfigured bool
	LastRoutineAt   *time.Time
	LastBackupAt    *time.Time
	External        *ExternalReport
	// ReachedThisServer: браузер открыл панель по имени из public_url и
	// получил из /probe метку этого процесса.
	ReachedThisServer bool
	TLS               *TLSInfo
	Now               time.Time
}

// Result is one row in the admin check list.
type Result struct {
	ID     string `json:"id"`
	Status Status `json:"status"`
	Title  string `json:"title"`
	Detail string `json:"detail"`
}

// RunChecks evaluates instance health and returns a fixed-order result list.
func RunChecks(ctx context.Context, in Input) []Result {
	now := in.Now.UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}
	in.Now = now
	return []Result{
		checkDomain(ctx, in),
		checkHTTPSOutside(in),
		checkHTTPRedirect(in),
		checkCertLE(in),
		checkCertChain(in),
		checkProxyClient(in),
		checkProxyBodyLimit(in),
		checkProxyHeaders(in),
		checkProxySSE(in),
		checkProxyTimeout(in),
		checkMail(in),
		checkVAPIDKeys(in),
		checkPWA(in),
		checkClocks(ctx, now),
		checkDiskSpace(in.DataDir),
		checkDailyRoutine(in, now),
		checkBackup(in, now),
	}
}

func checkClocks(ctx context.Context, _ time.Time) Result {
	r := Result{
		ID:     "clocks",
		Title:  "Часы",
		Status: StatusOK,
	}
	local := time.Now().UTC()
	drift, err := ntpDrift(ctx, local)
	if err != nil {
		r.Detail = fmt.Sprintf("серверное время %s; NTP недоступен", local.Format(time.RFC3339))
		return r
	}
	r.Detail = fmt.Sprintf("расходятся с NTP на %.1f с", drift.Seconds())
	if drift < 0 {
		drift = -drift
	}
	switch {
	case drift > 30*time.Second:
		r.Status = StatusFail
	case drift > 5*time.Second:
		r.Status = StatusWarn
	}
	return r
}

// DiskUsage returns free and total bytes for the filesystem containing path.
func DiskUsage(path string) (free, total uint64, err error) {
	return diskUsage(path)
}

func checkDiskSpace(dataDir string) Result {
	r := Result{ID: "disk_space", Title: "Место"}
	if dataDir == "" {
		r.Status = StatusWarn
		r.Detail = "каталог данных не задан"
		return r
	}
	free, total, err := DiskUsage(dataDir)
	if err != nil {
		r.Status = StatusWarn
		r.Detail = fmt.Sprintf("не удалось проверить: %v", err)
		return r
	}
	r.Detail = fmt.Sprintf("свободно %.1f ГБ из %.1f", bytesToGB(free), bytesToGB(total))
	switch {
	case free < 100*1024*1024:
		r.Status = StatusFail
	case free < 1024*1024*1024:
		r.Status = StatusWarn
	default:
		r.Status = StatusOK
	}
	return r
}

func checkMail(in Input) Result {
	r := Result{ID: "mail", Title: "Письмо"}
	if in.SMTPTestSentAt == nil || in.SMTPTestSentAt.IsZero() {
		r.Status = StatusWarn
		r.Detail = "ещё не отправлялось"
		return r
	}
	if strings.TrimSpace(in.SMTPLastError) != "" {
		r.Status = StatusFail
		r.Detail = "не ушло: " + strings.TrimSpace(in.SMTPLastError)
		return r
	}
	r.Status = StatusOK
	r.Detail = "ушло " + formatMailAgo(in.Now, *in.SMTPTestSentAt)
	return r
}

func formatMailAgo(now, then time.Time) string {
	d := now.UTC().Sub(then.UTC())
	if d < 0 {
		d = 0
	}
	mins := int(d.Minutes())
	if mins < 1 {
		return "только что"
	}
	if mins < 60 {
		return fmt.Sprintf("%d мин. назад", mins)
	}
	hours := int(d.Hours())
	if hours < 24 {
		return fmt.Sprintf("%d ч. назад", hours)
	}
	return then.UTC().Format("2.01.2006")
}

func checkVAPIDKeys(in Input) Result {
	r := Result{ID: "vapid_keys", Title: "VAPID-ключи"}
	if in.VAPIDConfigured {
		r.Status = StatusOK
		r.Detail = "ключи созданы"
		return r
	}
	r.Status = StatusWarn
	r.Detail = "ключи не заданы"
	return r
}

func checkPWA(in Input) Result {
	r := Result{ID: "pwa", Title: "Манифест и service worker"}
	if in.Loopback {
		r.Status = StatusNA
		r.Detail = "на loopback не применимо"
		return r
	}
	if in.External == nil {
		r.Status = StatusWarn
		r.Detail = "ожидается проверка из браузера"
		return r
	}
	if in.External.PWAOK {
		r.Status = StatusOK
		r.Detail = "манифест отдаётся, service worker зарегистрирован"
		return r
	}
	r.Status = StatusWarn
	r.Detail = "манифест или service worker недоступны"
	return r
}

func checkDailyRoutine(in Input, now time.Time) Result {
	r := Result{ID: "daily_routine", Title: "Суточная рутина"}
	if in.LastRoutineAt == nil || in.LastRoutineAt.IsZero() {
		r.Status = StatusWarn
		r.Detail = "ещё не выполнялась"
		return r
	}
	age := now.Sub(in.LastRoutineAt.UTC())
	if age > 36*time.Hour {
		r.Status = StatusWarn
		r.Detail = fmt.Sprintf("последний запуск %s", in.LastRoutineAt.UTC().Format(time.RFC3339))
		return r
	}
	r.Status = StatusOK
	r.Detail = fmt.Sprintf("последний запуск %s", in.LastRoutineAt.UTC().Format(time.RFC3339))
	return r
}

func checkBackup(in Input, now time.Time) Result {
	r := Result{ID: "backup", Title: "Бэкап"}
	if in.LastBackupAt == nil || in.LastBackupAt.IsZero() {
		r.Status = StatusWarn
		r.Detail = "ещё не создавалась"
		return r
	}
	age := now.Sub(in.LastBackupAt.UTC())
	if age > 7*24*time.Hour {
		r.Status = StatusWarn
		r.Detail = fmt.Sprintf("последняя копия %s", in.LastBackupAt.UTC().Format(time.RFC3339))
		return r
	}
	r.Status = StatusOK
	r.Detail = fmt.Sprintf("последняя копия %s", in.LastBackupAt.UTC().Format(time.RFC3339))
	return r
}

func checkHTTPSOutside(in Input) Result {
	r := Result{ID: "https_outside", Title: "HTTPS снаружи"}
	if in.Loopback {
		r.Status = StatusNA
		r.Detail = "на loopback не применимо"
		return r
	}
	if in.External == nil {
		r.Status = StatusWarn
		r.Detail = "ожидается проверка из браузера"
		return r
	}
	if !in.External.FromOutside {
		r.Status = StatusWarn
		r.Detail = "браузер не подтвердил доступ из другой сети"
		return r
	}
	if !in.External.HTTPSOK {
		r.Status = StatusFail
		r.Detail = "HTTPS снаружи недоступен"
		return r
	}
	r.Status = StatusOK
	r.Detail = fmt.Sprintf("200 за %d мс; ваш браузер вышел из другой сети", in.External.HTTPSMS)
	return r
}

func checkHTTPRedirect(in Input) Result {
	r := Result{ID: "http_redirect", Title: "HTTP → HTTPS"}
	if in.Loopback {
		r.Status = StatusNA
		r.Detail = "на loopback не применимо"
		return r
	}
	if !strings.HasPrefix(strings.ToLower(in.PublicURL), "https://") {
		r.Status = StatusWarn
		r.Detail = "public_url без HTTPS"
		return r
	}
	if !RedirectApplies(in.PublicURL) {
		r.Status = StatusNA
		r.Detail = "HTTPS на нестандартном порту — перенаправление с 80 не проверяется"
		return r
	}
	if in.External == nil {
		r.Status = StatusWarn
		r.Detail = "ожидается проверка сервера"
		return r
	}
	if in.External.RedirectPermanent {
		code := in.External.RedirectStatus
		if code == 0 {
			code = 308
		}
		r.Status = StatusOK
		r.Detail = fmt.Sprintf("%d, постоянный", code)
		return r
	}
	r.Status = StatusFail
	r.Detail = "HTTP не перенаправляет на HTTPS"
	return r
}

func checkProxyClient(in Input) Result {
	return proxyCheck(in, "proxy_client", "Настоящий адрес клиента", func(ext *ExternalReport) (Status, string) {
		xff := strings.TrimSpace(ext.XForwardedFor)
		xri := strings.TrimSpace(ext.XRealIP)
		if xff == "" && xri == "" {
			return StatusFail, "X-Forwarded-For не приходит: лимит запросов кода по адресу общий на всех"
		}
		ip := net.ParseIP(strings.TrimSpace(ext.ClientIP))
		if ip != nil && ip.IsLoopback() {
			return StatusFail, "сервер видит клиента как 127.0.0.1 — лимит кодов общий"
		}
		return StatusOK, "X-Forwarded-For доходит до Wynd"
	})
}

func checkProxyHeaders(in Input) Result {
	return proxyCheck(in, "proxy_headers", "Протокол", func(ext *ExternalReport) (Status, string) {
		if ext.ProxyHTTPS {
			return StatusOK, "X-Forwarded-Proto: https — ссылки в письмах верные"
		}
		return StatusFail, "ожидается X-Forwarded-Proto: https"
	})
}

func checkProxyBodyLimit(in Input) Result {
	return proxyCheck(in, "proxy_body_limit", "Потолок тела запроса", func(ext *ExternalReport) (Status, string) {
		if ext.ProxyBodyLimitOK {
			return StatusOK, "лимит достаточен для загрузок"
		}
		return StatusFail, "лимит тела запроса слишком мал — фото вернётся с 413"
	})
}

func checkProxySSE(in Input) Result {
	return proxyCheck(in, "proxy_sse", "Буферизация", func(ext *ExternalReport) (Status, string) {
		if ext.ProxySSEOK {
			return StatusOK, "выключена, SSE доходит сразу"
		}
		return StatusFail, "SSE буферизуется или обрывается"
	})
}

func checkProxyTimeout(in Input) Result {
	return proxyCheck(in, "proxy_timeout", "Таймаут", func(ext *ExternalReport) (Status, string) {
		sec := ext.ProxyReadTimeout
		if sec <= 0 {
			sec = proxy.ReadTimeoutSeconds
		}
		if !ext.ProxyStreamOK {
			return StatusFail, "прокси обрывает длинную отдачу — загрузка может не дойти"
		}
		return StatusOK, fmt.Sprintf("%d с, длинная загрузка не рвётся", sec)
	})
}

func proxyCheck(in Input, id, title string, eval func(*ExternalReport) (Status, string)) Result {
	r := Result{ID: id, Title: title}
	if in.Loopback {
		r.Status = StatusNA
		r.Detail = "на loopback не применимо"
		return r
	}
	if strings.HasPrefix(strings.ToLower(in.PublicURL), "http://") {
		r.Status = StatusWarn
		r.Detail = "public_url без HTTPS"
		return r
	}
	if in.External == nil {
		r.Status = StatusWarn
		r.Detail = "ожидается проверка из браузера"
		return r
	}
	r.Status, r.Detail = eval(in.External)
	return r
}

func bytesToGB(n uint64) float64 {
	return float64(n) / (1024 * 1024 * 1024)
}
