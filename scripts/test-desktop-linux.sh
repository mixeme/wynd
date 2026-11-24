#!/usr/bin/env bash
# Linux-ворота «Wynd Настольный» (план 69): проверки, сборка и smoke в Xvfb.

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

echo "[linux] frontend check"
(cd web && npx svelte-kit sync)
(cd cmd/wynd-desktop/frontend && npm run check)

# go:embed требует готовые web/dist и frontend/dist до go vet/test.
echo "[linux] build"
scripts/build-desktop-linux.sh

echo "[linux] go vet/test (desktop,gtk3, весь контур)"
go vet -tags "desktop,gtk3" ./...
go test -tags "desktop,gtk3" ./...

if command -v xvfb-run >/dev/null 2>&1; then
	echo "[linux] smoke в Xvfb"
	xvfb-run -a timeout 20s ./dist/wynd-desktop --smoke-test
else
	echo "[linux] xvfb-run не установлен — smoke пропущен" >&2
fi

echo "[linux] OK"
