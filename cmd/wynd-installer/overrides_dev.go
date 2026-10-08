//go:build dev

package main

import (
	"os"
	"strconv"

	"gitea.mixdep.ru/mix/wynd/internal/installer"
)

// Только в `wails dev`: окно можно направить на подставной SSH-сервер, не
// трогая настоящий ~/.ssh. В собранном приложении этих переменных нет.
//
//	WYND_INSTALLER_SSH_PORT — порт вместо 22
//	WYND_INSTALLER_SSH_DIR  — каталог вместо ~/.ssh (known_hosts и ключи)
//	WYND_INSTALLER_IMAGE    — файл образа Wynd (docker save), который уедет на сервер
func devPort() int {
	port, _ := strconv.Atoi(os.Getenv("WYND_INSTALLER_SSH_PORT"))
	return port
}

func devDialer() installer.Dialer {
	return installer.Dialer{SSHDir: os.Getenv("WYND_INSTALLER_SSH_DIR")}
}

func imageSource() installer.ImageSource {
	if path := os.Getenv("WYND_INSTALLER_IMAGE"); path != "" {
		return installer.TarImage{Path: path}
	}
	return nil
}
