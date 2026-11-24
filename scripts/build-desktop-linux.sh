#!/usr/bin/env bash
# Сборка Linux-версии «Wynd Настольный» (план 69).
#
# Требует GTK3/WebKit2GTK 4.1 и cgo:
#   sudo apt install build-essential pkg-config libgtk-3-dev \
#     libwebkit2gtk-4.1-dev libx11-dev nodejs npm
#
# Результат — dist/wynd-desktop (ELF, без самообновления).

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

for tool in go npm pkg-config; do
	if ! command -v "$tool" >/dev/null 2>&1; then
		echo "ERROR: $tool не найден в PATH" >&2
		exit 1
	fi
done

if ! pkg-config --exists gtk+-3.0 webkit2gtk-4.1; then
	echo "ERROR: нужны libgtk-3-dev и libwebkit2gtk-4.1-dev (pkg-config gtk+-3.0 webkit2gtk-4.1)" >&2
	exit 1
fi

if [ ! -d web/node_modules ]; then
	echo "Устанавливаю зависимости web..."
	(cd web && npm ci --no-audit --no-fund)
fi
if [ ! -d cmd/wynd-desktop/frontend/node_modules ]; then
	echo "Устанавливаю зависимости оболочки..."
	(cd cmd/wynd-desktop/frontend && npm ci --no-audit --no-fund)
fi

echo "Собираю клиент сервера (вшитый интерфейс)..."
(cd web && npx svelte-kit sync && npm run build)

echo "Собираю оболочку окна..."
(cd cmd/wynd-desktop/frontend && npm run build)

mkdir -p dist
echo "Собираю Linux-бинарник..."
CGO_ENABLED=1 GOOS=linux GOARCH=amd64 \
	go build -trimpath -buildvcs=false \
	-tags "desktop,production,gtk3" \
	-ldflags "-s -w" \
	-o dist/wynd-desktop ./cmd/wynd-desktop

ls -lh dist/wynd-desktop
