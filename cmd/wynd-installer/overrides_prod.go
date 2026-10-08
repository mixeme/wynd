//go:build (desktop || bindings) && !dev

package main

import "gitea.mixdep.ru/mix/wynd/internal/installer"

func devPort() int { return 0 }

func devDialer() installer.Dialer { return installer.Dialer{} }
