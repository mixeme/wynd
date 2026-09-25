package check

import (
	"context"
	"fmt"
	"strings"
	"time"
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
	HTTPSOK           bool `json:"https_ok"`
	HTTPSMS           int  `json:"https_ms"`
	RedirectPermanent bool `json:"redirect_permanent"`
	FromOutside       bool `json:"from_outside"`
	ProxyHTTPS        bool `json:"proxy_https"`
	ProxyBodyLimitOK  bool `json:"proxy_body_limit_ok"`
	ProxySSEOK        bool `json:"proxy_sse_ok"`
}

// Input drives RunChecks.
type Input struct {
	Loopback        bool
	PublicURL       string
	DataDir         string
	MailConfigured  bool
	SMTPTestSentAt  *time.Time
	VAPIDConfigured bool
	LastRoutineAt   *time.Time
	LastBackupAt    *time.Time
	External        *ExternalReport
	Now             time.Time
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
	return []Result{
		checkClocks(ctx, now),
		checkDiskSpace(in.DataDir),
		checkSMTP(in),
		checkDKIM(in),
		checkVAPIDKeys(in),
		checkDailyRoutine(in, now),
		checkBackup(in, now),
		checkHTTPSOutside(in),
		checkProxyHeaders(in),
		checkProxyBodyLimit(in),
		checkProxySSE(in),
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

func checkDiskSpace(dataDir string) Result {
	r := Result{ID: "disk_space", Title: "Место на диске"}
	if dataDir == "" {
		r.Status = StatusWarn
		r.Detail = "каталог данных не задан"
		return r
	}
	free, total, err := diskUsage(dataDir)
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

func checkSMTP(in Input) Result {
	r := Result{ID: "smtp", Title: "SMTP"}
	if !in.MailConfigured {
		r.Status = StatusWarn
		r.Detail = "SMTP не настроен"
		return r
	}
	if in.SMTPTestSentAt == nil || in.SMTPTestSentAt.IsZero() {
		r.Status = StatusWarn
		r.Detail = "тестовое письмо ещё не отправлялось"
		return r
	}
	r.Status = StatusOK
	r.Detail = fmt.Sprintf("тестовое письмо отправлено %s", in.SMTPTestSentAt.UTC().Format(time.RFC3339))
	return r
}

func checkDKIM(in Input) Result {
	r := Result{ID: "dkim", Title: "DKIM"}
	if in.Loopback {
		r.Status = StatusNA
		r.Detail = "на loopback не применимо"
		return r
	}
	if !in.MailConfigured {
		r.Status = StatusWarn
		r.Detail = "настройте SMTP, затем DKIM у провайдера почты"
		return r
	}
	r.Status = StatusOK
	r.Detail = "проверьте DKIM у почтового провайдера"
	return r
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
	r := Result{ID: "backup", Title: "Резервная копия"}
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
	r.Detail = fmt.Sprintf("200 за %d мс", in.External.HTTPSMS)
	return r
}

func checkProxyHeaders(in Input) Result {
	return proxyCheck(in, "proxy_headers", "Прокси: протокол", func(ext *ExternalReport) (Status, string) {
		if ext.ProxyHTTPS {
			return StatusOK, "X-Forwarded-Proto: https"
		}
		return StatusFail, "ожидается X-Forwarded-Proto: https"
	})
}

func checkProxyBodyLimit(in Input) Result {
	return proxyCheck(in, "proxy_body_limit", "Прокси: лимит тела", func(ext *ExternalReport) (Status, string) {
		if ext.ProxyBodyLimitOK {
			return StatusOK, "лимит достаточен для загрузок"
		}
		return StatusFail, "лимит тела запроса слишком мал"
	})
}

func checkProxySSE(in Input) Result {
	return proxyCheck(in, "proxy_sse", "Прокси: SSE", func(ext *ExternalReport) (Status, string) {
		if ext.ProxySSEOK {
			return StatusOK, "буферизация выключена, SSE доходит"
		}
		return StatusFail, "SSE буферизуется или обрывается"
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
