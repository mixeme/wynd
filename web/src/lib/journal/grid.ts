import { apiJson, isAccessError } from '$lib/api/client';
import { readCachedSnapshot, writeCachedSnapshot } from '$lib/api/snapshots';
import { tileBlobId } from './present';
import type { GridItem, GridSnapshot } from './types';

export async function fetchGrid(origin: string, circleId: string): Promise<GridSnapshot> {
	const data = await apiJson<GridSnapshot>(origin, `/circles/${circleId}/grid`);
	await writeCachedSnapshot(origin, 'grid', circleId, data);
	return data;
}

export async function loadGridCached(
	origin: string,
	circleId: string
): Promise<GridSnapshot | undefined> {
	return readCachedSnapshot<GridSnapshot>(origin, 'grid', circleId);
}

export async function loadGrid(origin: string, circleId: string): Promise<GridSnapshot> {
	try {
		return await fetchGrid(origin, circleId);
	} catch (err) {
		if (isAccessError(err)) throw err;
		const cached = await loadGridCached(origin, circleId);
		if (cached) return cached;
		throw err instanceof Error ? err : new Error('grid_unavailable');
	}
}

export interface GridTile {
	postId: string;
	blobId: string;
	entryDate: string;
	createdAt: string;
	photoCount: number;
	kind: 'photo' | 'video';
	/** Что рисует плитка: фото или кадр ролика (tileBlobId). */
	tileId: string;
}

/** Одна плитка на запись: обложка и число фото в посте. */
export function groupGridTiles(items: GridItem[]): GridTile[] {
	const byPost = new Map<string, GridItem[]>();
	for (const item of items) {
		const list = byPost.get(item.post_id) ?? [];
		list.push(item);
		byPost.set(item.post_id, list);
	}
	const tiles: GridTile[] = [];
	for (const photos of byPost.values()) {
		const cover = photos.find((p) => p.is_cover) ?? photos[0];
		tiles.push({
			postId: cover.post_id,
			blobId: cover.blob_id,
			entryDate: cover.entry_date,
			createdAt: cover.created_at,
			photoCount: photos.length,
			kind: cover.kind === 'video' ? 'video' : 'photo',
			tileId: tileBlobId({
				blob_id: cover.blob_id,
				kind: cover.kind ?? 'photo',
				video_poster_blob_id: cover.poster_blob_id
			})
		});
	}
	tiles.sort((a, b) => b.createdAt.localeCompare(a.createdAt));
	return tiles;
}
