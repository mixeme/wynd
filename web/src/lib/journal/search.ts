import { apiJson } from '$lib/api/client';
import type { CircleSearchHit, CircleSearchResponse } from './types';

export async function searchCircle(
	origin: string,
	circleId: string,
	query: string,
	limit = 50
): Promise<CircleSearchHit[]> {
	const params = new URLSearchParams({ q: query, limit: String(limit) });
	const data = await apiJson<CircleSearchResponse>(
		origin,
		`/circles/${circleId}/search?${params}`
	);
	return data.hits;
}
