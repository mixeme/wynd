import { apiJson } from '$lib/api/client';

/**
 * Порции ленты, «Сетки» и «Карты» (план 46, C18). Снимок отдаёт самые новые
 * и курсор `next_before`, пока старше есть ещё; старшие порции не кэшируются —
 * без сети догрузка просто не идёт.
 */
export async function fetchOlderPage<T>(
	origin: string,
	path: string,
	before: string
): Promise<T & { next_before?: string }> {
	return apiJson<T & { next_before?: string }>(origin, `${path}?before=${encodeURIComponent(before)}`);
}

/** Первая порция и догруженные старшие — одним списком, без повторов. */
export function mergePages<T>(first: T[], older: T[], key: (item: T) => string): T[] {
	if (!older.length) return first;
	const seen = new Set(first.map(key));
	return [...first, ...older.filter((item) => !seen.has(key(item)))];
}

/**
 * После обновления первой порции её граница сдвигается к новым записям, и
 * между ней и уже догруженным появилась бы дыра. Перечитываем старшие от
 * новой границы — столько же, сколько было догружено.
 */
export async function refetchOlder<T>(
	load: (before: string) => Promise<{ items: T[]; next?: string }>,
	from: string | undefined,
	want: number
): Promise<{ items: T[]; next?: string }> {
	const items: T[] = [];
	let next = from;
	while (next && items.length < want) {
		const page = await load(next);
		items.push(...page.items);
		next = page.next;
	}
	return { items, next };
}
