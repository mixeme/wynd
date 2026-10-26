import { apiFetch, apiJson, apiPath } from '$lib/api/client';
import { invalidateCircleSnapshots } from '$lib/api/snapshots';
import { openDownload } from '$lib/navigation/openDownload';
import type { QueueFile } from '$lib/idb/db';
import { CHUNK_SIZE } from '$lib/queue/queue';
import { sha256Hex } from '$lib/sha256';
import type { InstanceWithCompression, MediaSummary } from './types';

export async function fetchCompression(origin: string) {
	const info = await apiJson<InstanceWithCompression>(origin, '/instance');
	return info.compression;
}

export async function uploadBlob(
	origin: string,
	file: QueueFile,
	onProgress?: (received: number, total: number) => void
): Promise<string> {
	const session = await apiJson<{ id: string }>(origin, '/uploads', {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ expected_size: file.size, mime_type: file.type, filename: file.name })
	});

	let offset = 0;
	onProgress?.(0, file.size);
	while (offset < file.data.byteLength) {
		const end = Math.min(offset + CHUNK_SIZE, file.data.byteLength);
		const chunk = file.data.slice(offset, end);
		const res = await apiFetch(origin, `/uploads/${session.id}`, {
			method: 'PUT',
			headers: {
				'Content-Type': 'application/octet-stream',
				'Upload-Offset': String(offset)
			},
			body: chunk
		});
		const body = (await res.json()) as { received_bytes: number };
		offset = body.received_bytes;
		onProgress?.(offset, file.size);
	}

	const complete = await apiJson<{ id: string }>(origin, `/uploads/${session.id}/complete`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ sha256: await sha256Hex(file.data) })
	});
	return complete.id;
}

export interface CreatePostInput {
	body: string;
	entry_date: string;
	captured_at?: string;
	media: MediaSummary[];
}

export async function createPost(
	origin: string,
	circleId: string,
	input: CreatePostInput
): Promise<void> {
	await apiJson(origin, `/circles/${circleId}/posts`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(input)
	});
	await invalidateCircleSnapshots(origin, circleId);
}

export async function editPost(
	origin: string,
	circleId: string,
	postId: string,
	body: string,
	entry_date: string,
	media?: MediaSummary[]
): Promise<void> {
	const payload: {
		body: string;
		entry_date: string;
		cover_blob_id?: string;
		media?: MediaSummary[];
	} = {
		body,
		entry_date
	};
	if (media !== undefined) {
		payload.media = media;
	}
	await apiJson(origin, `/circles/${circleId}/posts/${postId}`, {
		method: 'PATCH',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(payload)
	});
	await invalidateCircleSnapshots(origin, circleId);
}

export async function deletePost(origin: string, circleId: string, postId: string): Promise<void> {
	await apiFetch(origin, `/circles/${circleId}/posts/${postId}`, { method: 'DELETE' });
	await invalidateCircleSnapshots(origin, circleId);
}

export async function createComment(
	origin: string,
	circleId: string,
	postId: string,
	body: string,
	clientId?: string
): Promise<void> {
	await apiJson(origin, `/circles/${circleId}/posts/${postId}/comments`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		// Ключ — чтобы повтор после оборванного ответа не создал вторую реплику.
		body: JSON.stringify({ body, client_id: clientId })
	});
	await invalidateCircleSnapshots(origin, circleId);
}

export async function editComment(
	origin: string,
	circleId: string,
	postId: string,
	commentId: string,
	body: string,
	/** Вложения целиком (C53); нет — меняются только слова. */
	media?: Array<{
		blob_id: string;
		kind: string;
		voice?: boolean;
		audio_duration_ms?: number;
		audio_peaks?: number[];
	}>
): Promise<void> {
	await apiJson(origin, `/circles/${circleId}/posts/${postId}/comments/${commentId}`, {
		method: 'PATCH',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(media ? { body, media } : { body })
	});
	await invalidateCircleSnapshots(origin, circleId);
}

export async function deleteComment(
	origin: string,
	circleId: string,
	postId: string,
	commentId: string
): Promise<void> {
	await apiFetch(origin, `/circles/${circleId}/posts/${postId}/comments/${commentId}`, {
		method: 'DELETE'
	});
	await invalidateCircleSnapshots(origin, circleId);
}

export async function setReaction(
	origin: string,
	circleId: string,
	postId: string,
	emoji: string
): Promise<void> {
	await apiJson(origin, `/circles/${circleId}/posts/${postId}/reactions`, {
		method: 'PUT',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ emoji })
	});
	await invalidateCircleSnapshots(origin, circleId);
}

export async function removeReaction(
	origin: string,
	circleId: string,
	postId: string
): Promise<void> {
	await apiFetch(origin, `/circles/${circleId}/posts/${postId}/reactions`, {
		method: 'DELETE'
	});
	await invalidateCircleSnapshots(origin, circleId);
}

/**
 * Архив участника (6.23, план 46, C38): приложение берёт у сервера
 * одноразовую ссылку, а качает сам браузер — со своим ходом и без копии
 * архива в памяти вкладки. Раньше fetch собирал весь ZIP в Blob, и запрос на
 * сохранение приходил только через минуты, а на телефоне гигабайты в памяти
 * могли уронить вкладку. Сервер отдаёт архив потоком, сразу с первым байтом.
 */
export async function downloadArchive(origin: string, downloadUrl: string): Promise<void> {
	const path = downloadUrl.replace(/^\/api\/v1/, '').replace(/\/download$/, '/link');
	const { url } = await apiJson<{ url: string }>(origin, path, { method: 'POST' });
	openDownload(apiPath(origin, url.replace(/^\/api\/v1/, '')));
}
