#!/usr/bin/env bash
set -euo pipefail

# Install Wynd binary, systemd unit, and print the bootstrap URL.
#
# Usage:
#   sudo ./deploy/install.sh --public-url https://example.org
#   sudo ./deploy/install.sh --from-source --public-url https://example.org
#
# Options:
#   --public-url URL   Public HTTPS address (written to config.json)
#   --own-proxy        Skip Caddy; print a reverse-proxy snippet in the log
#   --from-source      Build ./cmd/wynd from the repository root
#   --data-dir PATH    Data directory (default: /var/lib/wynd)
#   --bin PATH         Pre-built binary to install

INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
DATA_DIR="${DATA_DIR:-/var/lib/wynd}"
SERVICE_USER="${SERVICE_USER:-wynd}"
PUBLIC_URL=""
OWN_PROXY=0
FROM_SOURCE=0
BINARY=""

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Caddy, стоявший до скрипта, обслуживает чужие сайты: его Caddyfile не наш,
# перезаписать его — снять их с публикации. Такой случай = режим --own-proxy:
# скрипт печатает фрагмент конфигурации и ничего не трогает (DEP-2).
CADDY_PREINSTALLED=0
if command -v caddy >/dev/null 2>&1; then
	CADDY_PREINSTALLED=1
fi

usage() {
	# Строки 4–15 — шапка с Usage и списком опций (DEP-2).
	sed -n '4,15p' "$0" | sed 's/^# \{0,1\}//'
}

while [[ $# -gt 0 ]]; do
	case "$1" in
	--public-url)
		PUBLIC_URL="${2:-}"
		shift 2
		;;
	--own-proxy)
		OWN_PROXY=1
		shift
		;;
	--from-source)
		FROM_SOURCE=1
		shift
		;;
	--data-dir)
		DATA_DIR="${2:-}"
		shift 2
		;;
	--bin)
		BINARY="${2:-}"
		shift 2
		;;
	-h | --help)
		usage
		exit 0
		;;
	*)
		echo "Unknown option: $1" >&2
		usage >&2
		exit 1
		;;
	esac
done

if [[ "${EUID:-$(id -u)}" -ne 0 ]]; then
	echo "Run as root." >&2
	exit 1
fi

public_url_host() {
	local u="${1:-}"
	u="${u#https://}"
	u="${u#http://}"
	u="${u%%/*}"
	echo "$u"
}

is_loopback_url() {
	case "$1" in
	http://127.0.0.1 | http://127.0.0.1:* | https://127.0.0.1 | https://127.0.0.1:* | \
	http://localhost | http://localhost:* | https://localhost | https://localhost:* | \
	http://[::1] | http://[::1]:* | https://[::1] | https://[::1]:*)
		return 0
		;;
	esac
	return 1
}

install_caddy_for() {
	local url="$1"
	local host
	host="$(public_url_host "$url")"
	if [[ -z "$host" ]]; then
		return 0
	fi
	if ! command -v apt-get >/dev/null 2>&1; then
		echo "Caddy: apt-get not found — install Caddy manually or pass --own-proxy." >&2
		return 1
	fi
	apt-get update -qq
	apt-get install -y -qq caddy || {
		echo "Caddy package install failed — configure reverse proxy manually or pass --own-proxy." >&2
		return 1
	}
	local caddyfile=/etc/caddy/Caddyfile
	sed "s/example.org/$host/g" "$SCRIPT_DIR/proxy/Caddyfile" >"$caddyfile"
	systemctl enable caddy.service 2>/dev/null || true
	if ! systemctl reload caddy.service 2>/dev/null && ! systemctl restart caddy.service; then
		echo "Caddy failed to start — public instance would be unreachable. Fix Caddy or pass --own-proxy." >&2
		return 1
	fi
	if ! systemctl is-active --quiet caddy.service; then
		echo "Caddy is not active — public instance would be unreachable. Fix Caddy or pass --own-proxy." >&2
		return 1
	fi
	echo "Caddy configured for $host ($caddyfile)."
}

print_proxy_snippet() {
	local url="$1"
	local host
	host="$(public_url_host "$url")"
	if [[ -z "$host" ]]; then
		return 0
	fi
	echo ""
	echo "Reverse proxy (Caddy) — paste into your proxy config:"
	echo "---"
	sed "s/example.org/$host/g" "$SCRIPT_DIR/proxy/Caddyfile"
	echo "---"
}

if ! command -v systemctl >/dev/null 2>&1; then
	echo "systemd is required." >&2
	exit 1
fi

if ! id "$SERVICE_USER" >/dev/null 2>&1; then
	useradd --system --home "$DATA_DIR" --shell /usr/sbin/nologin "$SERVICE_USER"
fi

install -d -o "$SERVICE_USER" -g "$SERVICE_USER" -m 0750 "$DATA_DIR"
install -d -o "$SERVICE_USER" -g "$SERVICE_USER" -m 0750 \
	"$DATA_DIR/blobs" "$DATA_DIR/keys"

tmp_bin="$(mktemp)"
cleanup() {
	rm -f "$tmp_bin"
}
trap cleanup EXIT

if [[ -n "$BINARY" ]]; then
	cp "$BINARY" "$tmp_bin"
elif [[ "$FROM_SOURCE" -eq 1 ]]; then
	if ! command -v go >/dev/null 2>&1; then
		echo "go is required for --from-source." >&2
		exit 1
	fi
	if ! command -v npm >/dev/null 2>&1; then
		echo "npm is required for --from-source." >&2
		exit 1
	fi
	(
		cd "$REPO_ROOT/web"
		# Всегда: node_modules от прошлой версии даёт сборку не из этого
		# package-lock.json, и это не видно ни в логе, ни в бинаре (DEP-2).
		npm ci
		npm run build
	)
	(
		cd "$REPO_ROOT"
		CGO_ENABLED=0 go build -ldflags="-s -w" -o "$tmp_bin" ./cmd/wynd
	)
elif [[ -f "$SCRIPT_DIR/wynd" ]]; then
	cp "$SCRIPT_DIR/wynd" "$tmp_bin"
else
	echo "Binary not found. Use --from-source, --bin, or place wynd next to install.sh." >&2
	exit 1
fi

run_as_service_user() {
	if command -v runuser >/dev/null 2>&1; then
		runuser -u "$SERVICE_USER" -- "$@"
	else
		su -s /bin/sh -c "$(printf '%q ' "$@")" "$SERVICE_USER"
	fi
}

# Обновление: миграции схемы необратимы, откатиться можно только из копии.
# Копия снимается старым бинарём до замены; отказ копирования — отказ
# обновления, работающая версия при этом не тронута (DEP-2).
if [[ -x "$INSTALL_DIR/wynd" && -f "$DATA_DIR/wynd.db" ]]; then
	stamp="$(date +%Y%m%d-%H%M%S)"
	backup_dir="$DATA_DIR/backups/pre-update-$stamp"
	echo "Backing up data to $backup_dir before replacing the binary..."
	if ! run_as_service_user env WYND_DATA_DIR="$DATA_DIR" "$INSTALL_DIR/wynd" backup "$backup_dir"; then
		echo "Backup failed — install aborted, the running version is untouched." >&2
		echo "Fix the cause (disk space, permissions on $DATA_DIR/backups) and re-run." >&2
		exit 1
	fi
	cp -p "$INSTALL_DIR/wynd" "$backup_dir/wynd"
	echo "Previous binary saved as $backup_dir/wynd."
fi

install -m 0755 "$tmp_bin" "$INSTALL_DIR/wynd"

config_path="$DATA_DIR/config.json"
if [[ ! -f "$config_path" ]]; then
	if [[ -n "$PUBLIC_URL" ]]; then
		cat >"$config_path" <<EOF
{
  "listen": "127.0.0.1:7676",
  "public_url": "$PUBLIC_URL"
}
EOF
	else
		echo "Warning: --public-url not set; config.json uses loopback." >&2
		echo "Pass --public-url https://your.domain for a public instance." >&2
		cat >"$config_path" <<'EOF'
{
  "listen": "127.0.0.1:7676",
  "public_url": "http://127.0.0.1:7676"
}
EOF
	fi
	chown "$SERVICE_USER:$SERVICE_USER" "$config_path"
	chmod 0640 "$config_path"
elif [[ -n "$PUBLIC_URL" ]]; then
	echo "Config already exists at $config_path; not overwriting public_url." >&2
fi

unit_path=/etc/systemd/system/wynd.service
sed \
	-e "s|Environment=WYND_DATA_DIR=.*|Environment=WYND_DATA_DIR=$DATA_DIR|" \
	-e "s|ExecStart=.*|ExecStart=$INSTALL_DIR/wynd|" \
	-e "s|ReadWritePaths=.*|ReadWritePaths=$DATA_DIR|" \
	"$SCRIPT_DIR/systemd/wynd.service" >"$unit_path"

systemctl daemon-reload
systemctl enable wynd.service
systemctl restart wynd.service

if [[ -n "$PUBLIC_URL" ]] && ! is_loopback_url "$PUBLIC_URL"; then
	if [[ "$OWN_PROXY" -eq 1 || "$CADDY_PREINSTALLED" -eq 1 ]]; then
		if [[ "$OWN_PROXY" -eq 0 ]]; then
			echo "Caddy was already installed — /etc/caddy/Caddyfile left untouched." >&2
		fi
		print_proxy_snippet "$PUBLIC_URL"
	elif ! install_caddy_for "$PUBLIC_URL"; then
		echo "" >&2
		echo "Public URL is set but reverse proxy is missing." >&2
		echo "Wynd listens on loopback only — fix Caddy or pass --own-proxy and configure a proxy." >&2
		exit 1
	fi
fi

public_url="$PUBLIC_URL"
if [[ -z "$public_url" && -f "$config_path" ]]; then
	public_url="$(sed -n 's/.*"public_url"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$config_path" | head -n1)"
fi
if [[ -z "$public_url" ]]; then
	public_url="http://127.0.0.1:7676"
fi

token=""
for _ in $(seq 1 30); do
	if [[ -f "$DATA_DIR/keys/bootstrap" ]]; then
		token="$(tr -d '[:space:]' <"$DATA_DIR/keys/bootstrap")"
		break
	fi
	sleep 1
done

echo ""
echo "Wynd installed."
echo "Data dir: $DATA_DIR"
if [[ -n "$token" ]]; then
	echo "Bootstrap URL: ${public_url%/}/admin/bootstrap?token=$token"
else
	echo "Bootstrap URL not ready yet. Check: journalctl -u wynd -n 20 | grep 'bootstrap URL:'"
fi
echo ""
