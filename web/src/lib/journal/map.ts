import { apiJson, isAccessError } from '$lib/api/client';
import { readCachedSnapshot, writeCachedSnapshot } from '$lib/api/snapshots';
import type { MapSnapshot } from './types';

export async function fetchMap(origin: string, circleId: string): Promise<MapSnapshot> {
	const data = await apiJson<MapSnapshot>(origin, `/circles/${circleId}/map`);
	await writeCachedSnapshot(origin, 'map', circleId, data);
	return data;
}

export async function loadMapCached(
	origin: string,
	circleId: string
): Promise<MapSnapshot | undefined> {
	return readCachedSnapshot<MapSnapshot>(origin, 'map', circleId);
}

export async function loadMap(origin: string, circleId: string): Promise<MapSnapshot> {
	try {
		return await fetchMap(origin, circleId);
	} catch (err) {
		if (isAccessError(err)) throw err;
		const cached = await loadMapCached(origin, circleId);
		if (cached) return cached;
		throw err instanceof Error ? err : new Error('map_unavailable');
	}
}
