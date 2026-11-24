Wynd Настольный для Linux (x86_64)
==================================

Поддерживаются Ubuntu 24.04 и Debian 12 с GTK3/WebKit2GTK 4.1.

Установка .deb:
  sudo apt install ./wynd-desktop_<версия>_amd64.deb

Запуск AppImage:
  chmod +x Wynd-<версия>-x86_64.AppImage
  ./Wynd-<версия>-x86_64.AppImage
На Ubuntu 24.04 для AppImage может понадобиться libfuse2t64, на Debian 12 —
libfuse2. Без FUSE: ./Wynd-<версия>-x86_64.AppImage --appimage-extract-and-run.

Что нужно системе:
- libgtk-3-0, libwebkit2gtk-4.1-0, libjavascriptcoregtk-4.1-0, libsoup-3.0-0;
- xdg-utils — открытие ссылок и папок;
- GStreamer (plugins-base/good/libav) — видео и звук;
- gnome-keyring или KWallet — сохранение входов (Secret Service);
- демон уведомлений (org.freedesktop.Notifications);
- для значка в GNOME — расширение AppIndicator.

Самообновления в этой версии нет: новую версию ставят пакетом или заменой
AppImage вручную.
