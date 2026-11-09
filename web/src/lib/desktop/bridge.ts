// Мост настольной оболочки: окно живёт на wails.localhost, а сервер — на
// своём адресе. Оболочка вставляет window.__wyndDesktop в HTML сервера, чтобы
// плашки, ссылки и приглашения называли настоящий адрес, а запросы шли через
// Go-прокси окна. В браузере моста нет — всё остаётся как было.

export interface DesktopBridge {
	origin: string;
}

export function desktopBridge(): DesktopBridge | undefined {
	if (typeof window === 'undefined') return undefined;
	const bridge = (window as unknown as { __wyndDesktop?: DesktopBridge }).__wyndDesktop;
	return bridge && typeof bridge.origin === 'string' && bridge.origin ? bridge : undefined;
}

/** Настоящий адрес открытого сервера; в браузере — адрес страницы. */
export function pageOrigin(): string {
	const bridge = desktopBridge();
	if (bridge) return bridge.origin;
	return typeof window !== 'undefined' ? window.location.origin : 'https://local';
}
