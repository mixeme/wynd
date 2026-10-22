package check

import (
	"sort"
	"time"
)

// Пороги «Проверки» — одни и для строк панели, и для письма администратору о
// поломках (план 46, A10, ч. 2).
const (
	DiskLowBytes      uint64 = 1 << 30
	DiskCriticalBytes uint64 = 100 << 20
	RoutineMaxAge            = 36 * time.Hour
	BackupMaxAge             = 7 * 24 * time.Hour
)

// ServerFull — сервер упёрся в предел: новое в круги не добавить.
func ServerFull(used, limit int64) bool {
	return limit > 0 && used >= limit
}

// ServerNearlyFull — занято больше 90% предела («порог 90%» на шкале «Хранилища»).
func ServerNearlyFull(used, limit int64) bool {
	return limit > 0 && used*10 > limit*9
}

// TroubleKind — вид беды. Порядок — срочность: первая беда идёт первой и в
// теме письма.
type TroubleKind int

const (
	TroubleCertExpired TroubleKind = iota
	TroubleServerFull
	TroubleDiskLow
	TroubleCertNotRenewed
	TroubleServerNearlyFull
	TroubleBackupMissing
	TroubleBackupOld
	TroubleRoutineStale
)

// Trouble — одна беда сервера и числа к ней.
type Trouble struct {
	Kind TroubleKind
	// Used и Total — занятое и предел сервера, свободное и весь диск.
	Used, Total int64
	// At — срок сертификата, последняя резервная копия или запуск рутины.
	At time.Time
}

// TroubleInput — то, что сервер знает о себе без браузера администратора.
type TroubleInput struct {
	UsedBytes, LimitBytes int64
	// DiskTotal 0 — диск не прочитан, о нём не пишем.
	DiskFree, DiskTotal uint64
	// TLS nil — сертификат не проверяли (loopback, http).
	TLS           *TLSInfo
	LastBackupAt  *time.Time
	LastRoutineAt *time.Time
	Now           time.Time
}

// Troubles — беды сервера по срочности; пусто — писать не о чем.
func Troubles(in TroubleInput) []Trouble {
	now := in.Now.UTC()
	var out []Trouble
	switch {
	case ServerFull(in.UsedBytes, in.LimitBytes):
		out = append(out, Trouble{Kind: TroubleServerFull, Used: in.UsedBytes, Total: in.LimitBytes})
	case ServerNearlyFull(in.UsedBytes, in.LimitBytes):
		out = append(out, Trouble{Kind: TroubleServerNearlyFull, Used: in.UsedBytes, Total: in.LimitBytes})
	}
	if in.DiskTotal > 0 && in.DiskFree < DiskLowBytes {
		out = append(out, Trouble{Kind: TroubleDiskLow, Used: int64(in.DiskFree), Total: int64(in.DiskTotal)})
	}
	// Сертификат, который не прочитался, — не беда для письма: причину
	// (сеть, DNS) видно только в «Проверке».
	if in.TLS != nil && in.TLS.ProbeError == "" {
		switch {
		case CertExpired(in.TLS, now):
			out = append(out, Trouble{Kind: TroubleCertExpired, At: in.TLS.NotAfter})
		case CertNotRenewed(in.TLS, now):
			out = append(out, Trouble{Kind: TroubleCertNotRenewed, At: in.TLS.NotAfter})
		}
	}
	switch {
	case in.LastBackupAt == nil || in.LastBackupAt.IsZero():
		out = append(out, Trouble{Kind: TroubleBackupMissing})
	case now.Sub(in.LastBackupAt.UTC()) > BackupMaxAge:
		out = append(out, Trouble{Kind: TroubleBackupOld, At: *in.LastBackupAt})
	}
	// Рутины, которая ещё не запускалась, не бывает: письмо уходит после неё.
	if in.LastRoutineAt != nil && now.Sub(in.LastRoutineAt.UTC()) > RoutineMaxAge {
		out = append(out, Trouble{Kind: TroubleRoutineStale, At: *in.LastRoutineAt})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Kind < out[j].Kind })
	return out
}
