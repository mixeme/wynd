package desktop

import "strings"

// ShouldNotify — показывать ли всплывающее уведомление: человек его не видит
// только когда окно спрятано в трей или свёрнуто; на виду мост молчит —
// человек и так смотрит в Wynd. Правило применяет мост App.Notify (план 66).
func ShouldNotify(visible, minimised bool) bool {
	return !visible || minimised
}

// sanitizeToastText делает текст безопасным для CDATA-обёртки в шаблоне
// уведомления: последовательность «]]>» закрыла бы CDATA раньше времени и
// поломала XML всплывающего окна.
func sanitizeToastText(s string) string {
	return strings.ReplaceAll(s, "]]>", "]]&gt;")
}
