import { apiJson } from '$lib/api/client';
import { readCachedSnapshot, writeCachedSnapshot } from '$lib/api/snapshots';
import type { CirclesResponse } from '$lib/circles/circles';
import type { CircleDetail } from './types';

export async function fetchCircleDetail(origin: string, circleId: string): Promise<CircleDetail> {
	return apiJson<CircleDetail>(origin, `/circles/${circleId}`);
}

/**
 * Отметить прочитанное на сервере и в снимке улицы. Снимок не удаляем: он
 * нужен холодному открытию ссылки на запись офлайн (C108) — без него круг не
 * найти, и браузер уходил на пустую улицу. Прочитанное отмечаем прямо в
 * снимке: точка «нового» гаснет так же, как при перечитывании списка.
 */
export async function advanceReadCursor(
	origin: string,
	circleId: string,
	seq: number
): Promise<void> {
	if (seq <= 0) return;
	await apiJson(origin, `/circles/${circleId}/read_cursor`, {
		method: 'PUT',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ seq })
	});
	const cached = await readCachedSnapshot<CirclesResponse>(origin, 'circles', '_list');
	const row = cached?.circles.find((c) => c.id === circleId);
	if (!cached || !row) return;
	if (row.unread === 0 && row.last_read_seq >= seq) return;
	row.unread = 0;
	row.last_read_seq = Math.max(row.last_read_seq, seq);
	await writeCachedSnapshot(origin, 'circles', '_list', cached);
}
