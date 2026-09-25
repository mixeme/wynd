import { apiFetch, apiJson, isAccessError } from '$lib/api/client';
import { invalidateCircleSnapshots, readCachedSnapshot, writeCachedSnapshot } from '$lib/api/snapshots';
import type { DaySnapshot, DaysSnapshot } from './types';

export async function fetchDays(origin: string, circleId: string): Promise<DaysSnapshot> {
	const data = await apiJson<DaysSnapshot>(origin, `/circles/${circleId}/days`);
	await writeCachedSnapshot(origin, 'days', circleId, data);
	return data;
}

export async function loadDaysCached(
	origin: string,
	circleId: string
): Promise<DaysSnapshot | undefined> {
	return readCachedSnapshot<DaysSnapshot>(origin, 'days', circleId);
}

export async function loadDays(origin: string, circleId: string): Promise<DaysSnapshot> {
	try {
		return await fetchDays(origin, circleId);
	} catch (err) {
		if (isAccessError(err)) throw err;
		const cached = await loadDaysCached(origin, circleId);
		if (cached) return cached;
		throw err instanceof Error ? err : new Error('days_unavailable');
	}
}

export async function fetchDay(
	origin: string,
	circleId: string,
	entryDate: string
): Promise<DaySnapshot> {
	const data = await apiJson<DaySnapshot>(origin, `/circles/${circleId}/days/${entryDate}`);
	await writeCachedSnapshot(origin, 'day', `${circleId}:${entryDate}`, data);
	return data;
}

export async function loadDayCached(
	origin: string,
	circleId: string,
	entryDate: string
): Promise<DaySnapshot | undefined> {
	return readCachedSnapshot<DaySnapshot>(origin, 'day', `${circleId}:${entryDate}`);
}

export async function loadDay(
	origin: string,
	circleId: string,
	entryDate: string
): Promise<DaySnapshot> {
	try {
		return await fetchDay(origin, circleId, entryDate);
	} catch (err) {
		if (isAccessError(err)) throw err;
		const cached = await loadDayCached(origin, circleId, entryDate);
		if (cached) return cached;
		throw err instanceof Error ? err : new Error('day_unavailable');
	}
}

export async function setDayTitle(
	origin: string,
	circleId: string,
	entryDate: string,
	title: string
): Promise<void> {
	await apiJson(origin, `/circles/${circleId}/days/${entryDate}/title`, {
		method: 'PUT',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ title })
	});
	await invalidateCircleSnapshots(origin, circleId);
}

export async function clearDayTitle(
	origin: string,
	circleId: string,
	entryDate: string
): Promise<void> {
	await apiFetch(origin, `/circles/${circleId}/days/${entryDate}/title`, { method: 'DELETE' });
	await invalidateCircleSnapshots(origin, circleId);
}

export async function setDayCover(
	origin: string,
	circleId: string,
	entryDate: string,
	postId: string,
	blobId: string
): Promise<void> {
	await apiJson(origin, `/circles/${circleId}/days/${entryDate}/cover`, {
		method: 'PUT',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ post_id: postId, blob_id: blobId })
	});
	await invalidateCircleSnapshots(origin, circleId);
}

export async function clearDayCover(
	origin: string,
	circleId: string,
	entryDate: string
): Promise<void> {
	await apiFetch(origin, `/circles/${circleId}/days/${entryDate}/cover`, { method: 'DELETE' });
	await invalidateCircleSnapshots(origin, circleId);
}
