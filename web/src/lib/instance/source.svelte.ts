import { apiJson, normalizeOrigin } from '$lib/api/client';

// Адрес исходников (AGPL §13) приходит от сервера полем source_url в
// GET /instance: у каждого сервера он свой, форк меняет одну константу на
// сервере, а не пять мест в клиенте (LIC-2). Зашитый адрес — только запас
// на случай старого сервера, который поля ещё не отдаёт.
const FALLBACK = 'https://github.com/mixeme/wynd';

let urls = $state<Record<string, string>>({});
const inFlight = new Set<string>();

// Только веб-адрес: поле приходит от сервера, в том числе чужого (экран
// «Присоединиться» спрашивает /instance у любого адреса), и попадает в href.
// `javascript:` от такого сервера исполнился бы на origin этого клиента
// (аудит 2026-09-27).
function isWebUrl(url: string): boolean {
	try {
		const { protocol } = new URL(url);
		return protocol === 'https:' || protocol === 'http:';
	} catch {
		return false;
	}
}

// origin '' — тот же сервер, что отдал клиент (см. normalizeOrigin).
export function rememberSourceUrl(origin: string, url: string | undefined): void {
	if (typeof url !== 'string' || !isWebUrl(url)) return;
	const key = normalizeOrigin(origin);
	if (urls[key] === url) return;
	urls = { ...urls, [key]: url };
}

export function sourceUrl(origin: string = ''): string {
	return urls[normalizeOrigin(origin)] ?? FALLBACK;
}

// Для экранов, которые показывают ссылку, но сами /instance не запрашивают.
export function loadSourceUrl(origin: string = ''): void {
	const key = normalizeOrigin(origin);
	if (key in urls || inFlight.has(key)) return;
	inFlight.add(key);
	void apiJson<{ source_url?: string }>(key, '/instance')
		.then((info) => rememberSourceUrl(key, info.source_url))
		.catch(() => {})
		.finally(() => inFlight.delete(key));
}
