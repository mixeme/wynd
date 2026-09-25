/**
 * Разовые параметры адреса: `?lb=2`, `?reactions=<id>`, `?delete=1`,
 * `?dayPrompt=…`. Экраны собирали их руками — где `new URL(...)` и
 * `searchParams.delete`, где конкатенация строк (RDB-1).
 */

/** Число из параметра; нет или не число — fallback. */
export function numberParam(url: URL, name: string, fallback = -1): number {
	const raw = url.searchParams.get(name);
	if (raw === null || raw.trim() === '') return fallback;
	const n = Number(raw);
	return Number.isFinite(n) ? n : fallback;
}

/** Адрес того же экрана с добавленным параметром. */
export function withParam(pathname: string, name: string, value: string | number): string {
	const params = new URLSearchParams();
	params.set(name, String(value));
	return `${pathname}?${params}`;
}

/**
 * Адрес без указанного параметра; null — параметра и не было, значит
 * переходить некуда.
 */
export function withoutParam(url: URL, name: string): string | null {
	if (!url.searchParams.has(name)) return null;
	const next = new URL(url.href);
	next.searchParams.delete(name);
	return `${next.pathname}${next.search}${next.hash}`;
}
