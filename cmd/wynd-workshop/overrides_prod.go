//go:build (desktop || bindings) && !dev

package main

import "gitea.mixdep.ru/mix/wynd/internal/workshop"

func devPort() int { return 0 }

func devDialer() workshop.Dialer { return workshop.Dialer{} }

// Сервер скачивает Wynd из реестра выпусков — той же версии, что и установщик.
func imageSource() workshop.ImageSource {
	return workshop.RegistryImage{Repo: workshop.DefaultRegistry}
}

func devPassword() string { return "" }

func devTestCert() bool { return false }
