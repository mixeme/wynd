import { apiJson } from '$lib/api/client';
import { invalidateSnapshots } from '$lib/idb/db';
import type { CircleDetail } from './types';

export async function fetchCircleDetail(origin: string, circleId: string): Promise<CircleDetail> {
	return apiJson<CircleDetail>(origin, `/circles/${circleId}`);
}

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
	await invalidateSnapshots(origin, { kind: 'circles' });
}
