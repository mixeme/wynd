// Здесь нет маршрутизатора: QR открывается внутри принятого стартового экрана.
// FormLayout использует эти хуки только для сохранения прокрутки между маршрутами.
export function beforeNavigate() {}
export function afterNavigate() {}

// Общие диалоги используют стек «Назад» из navigation/up. На стартовом
// экране доступны только обычная история WebView и переход между документами.
export async function goto(url: string | URL, { replaceState = false } = {}) {
	if (replaceState) location.replace(url);
	else location.assign(url);
}

export function pushState(url: string | URL, state: object) {
	history.pushState(state, '', url);
}
