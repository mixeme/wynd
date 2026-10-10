//go:build (desktop || bindings) && !dev

package main

import "gitea.mixdep.ru/mix/wynd/internal/installer"

func devPort() int { return 0 }

func devDialer() installer.Dialer { return installer.Dialer{} }

// Сервер скачивает Wynd из реестра выпусков — той же версии, что и установщик.
func imageSource() installer.ImageSource {
	return installer.RegistryImage{Repo: installer.DefaultRegistry}
}

func devPassword() string { return "" }

func devTestCert() bool { return false }
