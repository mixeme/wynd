import { apiFetch, apiJson } from '$lib/api/client';
import { invalidateCircleSnapshots } from '$lib/api/snapshots';
import type { QueueFile } from '$lib/idb/db';
import { CHUNK_SIZE } from '$lib/queue/queue';
import type { InstanceWithCompression, MediaSummary } from './types';

async function sha256Hex(data: ArrayBuffer): Promise<string> {
	const hash = await crypto.subtle.digest('SHA-256', data);
	return [...new Uint8Array(hash)].map((b) => b.toString(16).padStart(2, '0')).join('');
}

export async function fetchCompression(origin: string) {
	const info = await apiJson<InstanceWithCompression>(origin, '/instance');
	return info.compression;
}

export async function uploadBlob(origin: string, file: QueueFile): Promise<string> {
	const session = await apiJson<{ id: string }>(origin, '/uploads', {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ expected_size: file.size, mime_type: file.type, filename: file.name })
	});

	let offset = 0;
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
	cover_blob_id?: string
): Promise<void> {
	const payload: { body: string; entry_date: string; cover_blob_id?: string } = {
		body,
		entry_date
	};
	if (cover_blob_id) payload.cover_blob_id = cover_blob_id;
	await apiJson(origin, `/circles/${circleId}/posts/${postId}`, {
		method: 'PATCH',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(payload)
	});
	await invalidateCircleSnapshots(origin, circleId);
}

export async function deletePost(
	origin: string,
	circleId: string,
	postId: string
): Promise<void> {
	await apiFetch(origin, `/circles/${circleId}/posts/${postId}`, { method: 'DELETE' });
	await invalidateCircleSnapshots(origin, circleId);
}

export async function createComment(
	origin: string,
	circleId: string,
	postId: string,
	body: string
): Promise<void> {
	await apiJson(origin, `/circles/${circleId}/posts/${postId}/comments`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ body })
	});
	await invalidateCircleSnapshots(origin, circleId);
}

export async function editComment(
	origin: string,
	circleId: string,
	postId: string,
	commentId: string,
	body: string
): Promise<void> {
	await apiJson(origin, `/circles/${circleId}/posts/${postId}/comments/${commentId}`, {
		method: 'PATCH',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ body })
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

export async function downloadArchive(
	origin: string,
	downloadUrl: string,
	layout?: 'feed' | 'posts'
): Promise<void> {
	let path = downloadUrl.replace(/^\/api\/v1/, '');
	if (layout) {
		const sep = path.includes('?') ? '&' : '?';
		path += `${sep}layout=${layout}`;
	}
	const res = await apiFetch(origin, path);
	const blob = await res.blob();
	const url = URL.createObjectURL(blob);
	const a = document.createElement('a');
	a.href = url;
	a.download = 'archive.zip';
	a.click();
	URL.revokeObjectURL(url);
}
