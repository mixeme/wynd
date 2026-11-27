#!/usr/bin/env bash
# Пакеты Linux-версии «Wynd Настольный» (план 69): .deb и AppImage.
# Требует уже собранный dist/wynd-desktop и dpkg-deb; AppImage — appimagetool
# (в CI скачивается закреплённой версии).

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

VERSION="$(sed -n 's/^const Version = "\(.*\)"/\1/p' internal/desktop/version.go | head -1)"
if [ -z "$VERSION" ]; then
	echo "ERROR: не прочитал desktop.Version" >&2
	exit 1
fi
if [ ! -x dist/wynd-desktop ]; then
	echo "ERROR: нет dist/wynd-desktop — сначала scripts/build-desktop-linux.sh" >&2
	exit 1
fi

BUILD="cmd/wynd-desktop/build/linux"
STAGE="dist/linux-package"
rm -rf "$STAGE"
mkdir -p "$STAGE"

install_common() {
	local root="$1"
	install -Dm755 dist/wynd-desktop "$root/usr/bin/wynd-desktop"
	install -Dm644 "$BUILD/wynd.desktop" "$root/usr/share/applications/wynd.desktop"
	install -Dm644 "$BUILD/wynd.metainfo.xml" "$root/usr/share/metainfo/wynd.metainfo.xml"
	install -Dm644 "$BUILD/README-linux.txt" "$root/usr/share/doc/wynd-desktop/README-linux.txt"
	for size in 48 64 128 256; do
		install -Dm644 "$BUILD/icons/hicolor/${size}x${size}/apps/wynd.png" \
			"$root/usr/share/icons/hicolor/${size}x${size}/apps/wynd.png"
	done
}

# .deb
DEB="$STAGE/wynd-desktop_${VERSION}_amd64"
install_common "$DEB"
mkdir -p "$DEB/DEBIAN"
cat > "$DEB/DEBIAN/control" <<EOF
Package: wynd-desktop
Version: $VERSION
Section: net
Priority: optional
Architecture: amd64
Maintainer: Wynd
Description: Wynd Настольный — окно к своему серверу Wynd
Depends: libgtk-3-0 | libgtk-3-0t64, libwebkit2gtk-4.1-0, libjavascriptcoregtk-4.1-0, libsoup-3.0-0, xdg-utils
Recommends: gstreamer1.0-plugins-base, gstreamer1.0-plugins-good, gstreamer1.0-libav, gnome-keyring, xdg-desktop-portal, pkexec
EOF
cat > "$DEB/DEBIAN/postinst" <<'EOF'
#!/bin/sh
set -e
if command -v update-desktop-database >/dev/null 2>&1; then
	update-desktop-database -q /usr/share/applications || true
fi
if command -v update-mime-database >/dev/null 2>&1; then
	update-mime-database /usr/share/mime || true
fi
exit 0
EOF
cp "$DEB/DEBIAN/postinst" "$DEB/DEBIAN/prerm"
chmod 755 "$DEB/DEBIAN/postinst" "$DEB/DEBIAN/prerm"
dpkg-deb --root-owner-group --build "$DEB" "dist/wynd-desktop_${VERSION}_amd64.deb"

# AppImage
APPDIR="$STAGE/Wynd.AppDir"
install_common "$APPDIR"
install -Dm644 "$BUILD/wynd.desktop" "$APPDIR/wynd.desktop"
install -Dm644 "$BUILD/icons/hicolor/256x256/apps/wynd.png" "$APPDIR/wynd.png"
cat > "$APPDIR/AppRun" <<'EOF'
#!/bin/sh
HERE="$(dirname "$(readlink -f "$0")")"
exec "$HERE/usr/bin/wynd-desktop" "$@"
EOF
chmod 755 "$APPDIR/AppRun"

APPIMAGETOOL="${APPIMAGETOOL:-appimagetool}"
if ! command -v "$APPIMAGETOOL" >/dev/null 2>&1; then
	echo "ERROR: appimagetool не найден (APPIMAGETOOL=... или в PATH)" >&2
	exit 1
fi
APPIMAGE_EXTRACT_AND_RUN=1 "$APPIMAGETOOL" --no-appstream "$APPDIR" \
	"dist/Wynd-${VERSION}-x86_64.AppImage"

(
	cd dist
	sha256sum "wynd-desktop_${VERSION}_amd64.deb" "Wynd-${VERSION}-x86_64.AppImage" > SHA256SUMS
)

ls -lh "dist/wynd-desktop_${VERSION}_amd64.deb" "dist/Wynd-${VERSION}-x86_64.AppImage" dist/SHA256SUMS
