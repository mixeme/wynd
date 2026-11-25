package auth

import (
	"strings"
)

// DeviceLabel — короткая человекочитаемая метка устройства из User-Agent
// (план 72). Сырую строку UA сервер не хранит, только эту метку.
// Наши оболочки помечают себя маркерами «WyndAndroid»/«WyndDesktop»
// (этапы 2–4); обычные браузеры разбираются по типу и системе.
func DeviceLabel(ua string) string {
	ua = strings.TrimSpace(ua)
	if ua == "" {
		return ""
	}
	switch {
	case strings.Contains(ua, "WyndAndroid"), strings.Contains(ua, "; wv) "):
		return "Wynd Android"
	case strings.Contains(ua, "WyndDesktop"):
		return "Wynd Настольный"
	case strings.Contains(ua, "WyndWorkshop"):
		return "Wynd Мастерская"
	}
	browser := browserLabel(ua)
	os := osLabel(ua)
	switch {
	case browser == "" && os == "":
		return "Браузер"
	case browser == "":
		return os
	case os == "":
		return browser
	default:
		return browser + " на " + os
	}
}

// browserLabel — тип браузера. Edge проверяется раньше Chrome: его UA
// содержит «Edg/» и «Chrome/» одновременно.
func browserLabel(ua string) string {
	switch {
	case strings.Contains(ua, "Firefox"):
		return "Firefox"
	case strings.Contains(ua, "Edg/"):
		return "Edge"
	case strings.Contains(ua, "OPR/"), strings.Contains(ua, "Opera"):
		return "Opera"
	case strings.Contains(ua, "Chrome"):
		return "Chrome"
	case strings.Contains(ua, "Safari"):
		return "Safari"
	default:
		return ""
	}
}

// osLabel — система из UA.
func osLabel(ua string) string {
	switch {
	case strings.Contains(ua, "Windows NT"), strings.Contains(ua, "Windows Phone"):
		return "Windows"
	case strings.Contains(ua, "Android"):
		return "Android"
	case strings.Contains(ua, "iPhone"):
		return "iPhone"
	case strings.Contains(ua, "iPad"):
		return "iPad"
	case strings.Contains(ua, "CrOS"):
		return "ChromeOS"
	case strings.Contains(ua, "Mac OS X"):
		return "macOS"
	case strings.Contains(ua, "Linux"):
		return "Linux"
	default:
		return ""
	}
}
