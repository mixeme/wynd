import { apiFetch } from '$lib/api/client';
import { getMedia, mediaKey, putMedia } from '$lib/idb/db';

const urlCache = new Map<string, string>();

export async function getMediaUrl(origin: string, blobId: string): Promise<string> {
	const key = mediaKey(origin, blobId);
	const cached = urlCache.get(key);
	if (cached) return cached;

	const stored = await getMedia(key);
	if (stored) {
		const url = URL.createObjectURL(new Blob([stored.buffer], { type: stored.mime }));
		urlCache.set(key, url);
		return url;
	}

	const res = await apiFetch(origin, `/blobs/${blobId}`);
	const buffer = await res.arrayBuffer();
	const mime = res.headers.get('Content-Type') || 'application/octet-stream';
	await putMedia(key, { buffer, mime });
	const url = URL.createObjectURL(new Blob([buffer], { type: mime }));
	urlCache.set(key, url);
	return url;
}

export async function downloadBlob(
	origin: string,
	blobId: string,
	filename: string
): Promise<void> {
	const url = await getMediaUrl(origin, blobId);
	const a = document.createElement('a');
	a.href = url;
	a.download = filename;
	a.click();
}

export function revokeMediaUrl(origin: string, blobId: string): void {
	const key = mediaKey(origin, blobId);
	const url = urlCache.get(key);
	if (url) {
		URL.revokeObjectURL(url);
		urlCache.delete(key);
	}
}

/** Cache a blob we just uploaded so the UI can show it without a stale entry. */
export function seedMediaUrl(
	origin: string,
	blobId: string,
	data: ArrayBuffer,
	mime: string
): string {
	revokeMediaUrl(origin, blobId);
	const key = mediaKey(origin, blobId);
	const url = URL.createObjectURL(new Blob([data], { type: mime }));
	urlCache.set(key, url);
	void putMedia(key, { buffer: data, mime });
	return url;
}
