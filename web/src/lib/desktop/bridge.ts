// Мост настольной оболочки: окно живёт на wails.localhost, а сервер — на
// своём адресе. Оболочка вставляет window.__wyndDesktop в HTML сервера, чтобы
// плашки, ссылки и приглашения называли настоящий адрес, а запросы шли через
// Go-прокси окна. В браузере моста нет — всё остаётся как было.

export interface DesktopBridge {
	origin: string;
	/**
	 * Локальный порт окна для API и потока событий. WebView2 буферизует
	 * ответы собственных запросов, и SSE через него не течёт — запросы
	 * клиента идут сюда обычным TCP. Пусто в старых сборках окна.
	 */
	apiBase?: string;
	/**
	 * Интерфейс вшит в окно (план 66, функция 9): страницы пришли из exe, а
	 * origin — выбранный сервер для данных. Не задано на страницах сервера.
	 */
	embedded?: boolean;
}

export function desktopBridge(): DesktopBridge | undefined {
	if (typeof window === 'undefined') return undefined;
	const bridge = (window as unknown as { __wyndDesktop?: DesktopBridge }).__wyndDesktop;
	return bridge && typeof bridge.origin === 'string' && bridge.origin ? bridge : undefined;
}

/** Локальный адрес API окна; в браузере и старых сборках — undefined. */
export function desktopApiBase(): string | undefined {
	const bridge = desktopBridge();
	return bridge?.apiBase || undefined;
}

/** Настоящий адрес открытого сервера; в браузере — адрес страницы. */
export function pageOrigin(): string {
	const bridge = desktopBridge();
	if (bridge) return bridge.origin;
	return typeof window !== 'undefined' ? window.location.origin : 'https://local';
}

/** Адрес — свой для этого клиента? В окне — настоящий адрес сервера, а не окна. */
export function pageOwnOrigin(origin: string): boolean {
	if (!origin) return false;
	if (typeof window !== 'undefined' && origin === window.location.origin) return true;
	return origin === pageOrigin();
}

/**
 * Путь Go-прокси для чужого сервера: /__srv/<scheme>/<host>. Для выбранного
 * сервера и адреса страницы прокси не нужен — они и так в корне окна.
 */
export function desktopProxyBase(origin: string): string | null {
	const bridge = desktopBridge();
	if (!bridge || !origin) return null;
	if (origin === bridge.origin || origin === window.location.origin) return null;
	try {
		const url = new URL(origin);
		if (url.protocol !== 'http:' && url.protocol !== 'https:') return null;
		return `/__srv/${url.protocol.slice(0, -1)}/${url.host}`;
	} catch {
		return null;
	}
}
