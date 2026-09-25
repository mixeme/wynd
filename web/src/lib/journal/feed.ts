import { apiJson, isAccessError } from '$lib/api/client';
import { readCachedSnapshot, writeCachedSnapshot } from '$lib/api/snapshots';
import type { FeedSnapshot } from './types';

export async function fetchFeed(origin: string, circleId: string): Promise<FeedSnapshot> {
	const data = await apiJson<FeedSnapshot>(origin, `/circles/${circleId}/feed`);
	await writeCachedSnapshot(origin, 'feed', circleId, data);
	return data;
}

export async function loadFeedCached(
	origin: string,
	circleId: string
): Promise<FeedSnapshot | undefined> {
	return readCachedSnapshot<FeedSnapshot>(origin, 'feed', circleId);
}

export async function loadFeed(
	origin: string,
	circleId: string
): Promise<FeedSnapshot> {
	try {
		return await fetchFeed(origin, circleId);
	} catch (err) {
		if (isAccessError(err)) throw err;
		const cached = await loadFeedCached(origin, circleId);
		if (cached) return cached;
		throw err instanceof Error ? err : new Error('feed_unavailable');
	}
}
