module gitea.mixdep.ru/mix/wynd

go 1.27.3

// В web/node_modules попадаются каталоги с .go-файлами (flatted), и ./... их
// подхватывает. Директива убирает их из любых шаблонов пакетов (TST-6).
ignore ./web/node_modules

require (
	filippo.io/age v1.2.1
	github.com/SherClockHolmes/webpush-go v1.4.0
	github.com/google/uuid v1.6.0
	github.com/wailsapp/wails/v3 v3.1.0
	github.com/zalando/go-keyring v0.2.8
	golang.org/x/crypto v0.57.0
	golang.org/x/sys v0.48.0
	modernc.org/sqlite v1.44.3
)

require (
	github.com/coder/websocket v1.8.14 // indirect
	github.com/danieljoos/wincred v1.2.3 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/ebitengine/purego v0.11.1 // indirect
	github.com/go-ole/go-ole v1.3.0 // indirect
	github.com/godbus/dbus/v5 v5.2.2 // indirect
	github.com/golang-jwt/jwt/v5 v5.2.2 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/ncruces/go-strftime v1.0.0 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	golang.org/x/exp v0.0.0-20260410095643-746e56fc9e2f // indirect
	golang.org/x/term v0.46.0 // indirect
	modernc.org/libc v1.67.6 // indirect
	modernc.org/mathutil v1.7.1 // indirect
	modernc.org/memory v1.11.0 // indirect
)
