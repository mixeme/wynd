//go:build (desktop || bindings) && !dev

package main

import "gitea.mixdep.ru/mix/wynd/internal/installer"

func devPort() int { return 0 }

func devDialer() installer.Dialer { return installer.Dialer{} }

// Реестра образов у Wynd пока нет: собранный установщик не знает, откуда
// сервер возьмёт Wynd, и шаг «Wynd» честно откажет (план 45, «До выпуска»).
func imageSource() installer.ImageSource { return nil }

func devPassword() string { return "" }
