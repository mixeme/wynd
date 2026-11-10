/**
 * Куда человек шёл, когда его увели на вход (владелец, 2026-10-23: «чтобы он
 * после входа попал туда, куда шёл. Это же касается вообще всех ссылок, когда
 * нужен вход»). Ссылка из письма открывает браузер без входа: экран уводит на
 * «Войти», а после кода — сюда, а не в список кругов.
 *
 * Для панели — свой ключ: вход участника и вход администратора не должны
 * подменять друг друга, и путь панели после кода участнику не по адресу.
 *
 * sessionStorage: переживает перезагрузку вкладки и не уходит в другие вкладки.
 * Через полчаса забываем — человек мог уже передумать.
 */
import { goto } from '$app/navigation';

const STORE = 'wynd:after-login';
const ADMIN_STORE = 'wynd:after-admin-login';
const TTL_MS = 30 * 60 * 1000;

/** Только свои экраны: путь от корня, не вход и не список кругов. */
export function isWantedPath(path: string): boolean {
	if (!path.startsWith('/') || path.startsWith('//') || path.includes('\\')) return false;
	const bare = path.split(/[?#]/)[0];
	return (
		bare !== '/' && bare !== '/circles' && !bare.startsWith('/auth') && !bare.startsWith('/admin')
	);
}

export function rememberWantedPath(path: string, now = Date.now()): void {
	if (!isWantedPath(path)) return;
	try {
		sessionStorage.setItem(STORE, JSON.stringify({ path, at: now }));
	} catch {
		/* нет хранилища — после входа будет список кругов */
	}
}

/** Путь, куда вернуть после входа, — один раз. */
export function takeWantedPath(now = Date.now()): string | undefined {
	try {
		const raw = sessionStorage.getItem(STORE);
		sessionStorage.removeItem(STORE);
		if (!raw) return undefined;
		const { path, at } = JSON.parse(raw) as { path?: unknown; at?: unknown };
		if (typeof path !== 'string' || typeof at !== 'number' || now - at > TTL_MS) return undefined;
		return isWantedPath(path) ? path : undefined;
	} catch {
		return undefined;
	}
}

/** Входа нет: запоминаем этот экран и ведём на «Войти». */
export function goToLogin(): void {
	rememberWantedPath(window.location.pathname + window.location.search);
	void goto('/');
}

/**
 * Экраны только панели: «/admin/login» и «/admin/bootstrap» — сами формы, и
 * голая «/admin» — то, куда и так вернули бы (нужна только с разделом из
 * письма, как «/admin#requests» с «Просят больше»).
 */
export function isWantedAdminPath(path: string): boolean {
	if (!path.startsWith('/') || path.startsWith('//') || path.includes('\\')) return false;
	const bare = path.split(/[?#]/)[0];
	if (!bare.startsWith('/admin')) return false;
	if (bare !== '/admin') return bare !== '/admin/login' && bare !== '/admin/bootstrap';
	return path.length > bare.length && path[bare.length] === '#';
}

export function rememberAdminPath(path: string, now = Date.now()): void {
	if (!isWantedAdminPath(path)) return;
	try {
		sessionStorage.setItem(ADMIN_STORE, JSON.stringify({ path, at: now }));
	} catch {
		/* нет хранилища — после входа будет та же панель */
	}
}

/** Путь в панели, куда вернуть после входа, — один раз, вместе с хэшем. */
export function takeAdminPath(now = Date.now()): string | undefined {
	try {
		const raw = sessionStorage.getItem(ADMIN_STORE);
		sessionStorage.removeItem(ADMIN_STORE);
		if (!raw) return undefined;
		const { path, at } = JSON.parse(raw) as { path?: unknown; at?: unknown };
		if (typeof path !== 'string' || typeof at !== 'number' || now - at > TTL_MS) return undefined;
		return isWantedAdminPath(path) ? path : undefined;
	} catch {
		return undefined;
	}
}
