import { ApiError, apiFetch, apiJson } from '$lib/api/client';
import { invalidateCircleSnapshots } from '$lib/api/snapshots';
import {
	addQueueItem,
	deleteQueueItem,
	getQueueItem,
	listQueueForCircle,
	listQueueItems,
	putQueueItem,
	type CommentQueuePayload,
	type PostQueuePayload,
	type QueueFile,
	type QueueItemType,
	type QueueMediaMeta,
	type QueueRecord,
	type QueueRecordWithId,
	type QueueUploadProgress,
	type ReactionQueuePayload
} from '$lib/idb/db';

export const CHUNK_SIZE = 1024 * 1024;

export interface QueuedPostView {
	id: number;
	body: string;
	entry_date: string;
	file_count: number;
	state: QueueRecord['state'];
	error?: string;
}

export interface QueuedCommentView {
	id: number;
	post_id: string;
	body: string;
	state: QueueRecord['state'];
	error?: string;
}

type QueueListener = () => void;

const listeners = new Set<QueueListener>();
let draining: Promise<void> | undefined;

function notify(): void {
	for (const listener of listeners) {
		listener();
	}
}

export function subscribeQueue(listener: QueueListener): () => void {
	listeners.add(listener);
	return () => listeners.delete(listener);
}

async function sha256Hex(data: ArrayBuffer): Promise<string> {
	const hash = await crypto.subtle.digest('SHA-256', data);
	return [...new Uint8Array(hash)].map((b) => b.toString(16).padStart(2, '0')).join('');
}

async function getUploadOffset(origin: string, sessionId: string): Promise<number> {
	const res = await apiFetch(origin, `/uploads/${sessionId}`, { method: 'HEAD' });
	const offset = res.headers.get('Upload-Offset');
	return offset ? parseInt(offset, 10) : 0;
}

async function uploadFile(
	origin: string,
	file: QueueFile,
	progress?: QueueUploadProgress
): Promise<string> {
	let sessionId = progress?.session_id;
	let offset = 0;

	if (sessionId) {
		offset = await getUploadOffset(origin, sessionId);
	} else {
		const session = await apiJson<{ id: string }>(origin, '/uploads', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ expected_size: file.size, mime_type: file.type, filename: file.name })
		});
		sessionId = session.id;
	}

	while (offset < file.data.byteLength) {
		const end = Math.min(offset + CHUNK_SIZE, file.data.byteLength);
		const chunk = file.data.slice(offset, end);
		const res = await apiFetch(origin, `/uploads/${sessionId}`, {
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

	const complete = await apiJson<{ id: string }>(origin, `/uploads/${sessionId}/complete`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ sha256: await sha256Hex(file.data) })
	});

	return complete.id;
}

async function submitQueueItem(item: QueueRecordWithId): Promise<void> {
	const { origin, circle_id: circleId, type, payload, files } = item;

	if (type === 'post') {
		const postPayload = payload as PostQueuePayload;
		const blobIds: string[] = [];
		let uploads = [...(item.uploads ?? [])];

		for (let i = 0; i < files.length; i++) {
			let progress = uploads.find((u) => u.file_index === i);
			if (progress?.blob_id) {
				blobIds.push(progress.blob_id);
				continue;
			}
			if (!progress) {
				const session = await apiJson<{ id: string }>(origin, '/uploads', {
					method: 'POST',
					headers: { 'Content-Type': 'application/json' },
					body: JSON.stringify({
						expected_size: files[i].size,
						mime_type: files[i].type
					})
				});
				progress = { file_index: i, session_id: session.id };
				uploads = [...uploads.filter((u) => u.file_index !== i), progress];
				await putQueueItem(item.id, { ...item, state: 'uploading', uploads });
			}
			const blobId = await uploadFile(origin, files[i], progress);
			progress = { ...progress, blob_id: blobId };
			uploads = [...uploads.filter((u) => u.file_index !== i), progress];
			await putQueueItem(item.id, { ...item, state: 'uploading', uploads });
			blobIds.push(blobId);
		}

		const media = blobIds.map((blob_id, i) => {
			const meta: QueueMediaMeta = postPayload.media_meta?.[i] ?? { kind: 'photo' };
			return {
				blob_id,
				kind: meta.kind,
				captured_at: meta.captured_at,
				geo_lat: meta.geo_lat,
				geo_lng: meta.geo_lng,
				is_cover: meta.is_cover ?? false
			};
		});

		await apiJson(origin, `/circles/${circleId}/posts`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({
				body: postPayload.body,
				entry_date: postPayload.entry_date,
				captured_at: postPayload.captured_at,
				media
			})
		});
		return;
	}

	if (type === 'comment') {
		const commentPayload = payload as CommentQueuePayload;
		await apiJson(
			origin,
			`/circles/${circleId}/posts/${commentPayload.post_id}/comments`,
			{
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ body: commentPayload.body })
			}
		);
		return;
	}

	if (type === 'reaction') {
		const reactionPayload = payload as ReactionQueuePayload;
		await apiJson(
			origin,
			`/circles/${circleId}/posts/${reactionPayload.post_id}/reactions`,
			{
				method: 'PUT',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ emoji: reactionPayload.emoji })
			}
		);
		return;
	}

	if (type === 'reaction_remove') {
		const reactionPayload = payload as ReactionQueuePayload;
		await apiFetch(
			origin,
			`/circles/${circleId}/posts/${reactionPayload.post_id}/reactions`,
			{ method: 'DELETE' }
		);
	}
}

async function processItem(item: QueueRecordWithId): Promise<boolean> {
	await putQueueItem(item.id, { ...item, state: 'uploading', error: undefined });
	notify();

	try {
		await submitQueueItem(item);
		await deleteQueueItem(item.id);
		await invalidateCircleSnapshots(item.origin, item.circle_id);
		notify();
		return true;
	} catch (err) {
		const stored = await getQueueItem(item.id);
		const latest = stored ? { ...stored, id: item.id } : item;
		if (err instanceof ApiError && err.status >= 400 && err.status < 500) {
			await putQueueItem(item.id, {
				...latest,
				state: 'failed',
				error: err.code
			});
			notify();
			return false;
		}

		await putQueueItem(item.id, { ...latest, state: 'pending' });
		notify();
		throw err;
	}
}

export async function drainQueue(): Promise<void> {
	if (draining) return draining;

	const run = async () => {
		while (navigator.onLine) {
			const items = await listQueueItems();
			const pending = items
				.filter((item) => item.state === 'pending' || item.state === 'uploading')
				.sort((a, b) => a.created_at - b.created_at);
			if (!pending.length) break;
			try {
				await processItem(pending[0]);
			} catch {
				break;
			}
		}
	};

	const promise = run().finally(() => {
		if (draining === promise) draining = undefined;
	});
	draining = promise;
	return promise;
}

export function initQueueDrain(): () => void {
	const onOnline = () => void drainQueue();
	window.addEventListener('online', onOnline);
	if (navigator.onLine) void drainQueue();
	return () => window.removeEventListener('online', onOnline);
}

async function enqueue(
	type: QueueItemType,
	origin: string,
	circleId: string,
	payload: QueueRecord['payload'],
	files: QueueFile[] = []
): Promise<number> {
	const id = await addQueueItem({
		type,
		origin,
		circle_id: circleId,
		payload,
		files,
		state: 'pending',
		created_at: Date.now()
	});
	notify();
	if (navigator.onLine) void drainQueue();
	return id;
}

export async function enqueuePost(
	origin: string,
	circleId: string,
	payload: PostQueuePayload,
	files: QueueFile[] = []
): Promise<number> {
	return enqueue('post', origin, circleId, payload, files);
}

export async function enqueueComment(
	origin: string,
	circleId: string,
	payload: CommentQueuePayload
): Promise<number> {
	return enqueue('comment', origin, circleId, payload);
}

export async function enqueueReaction(
	origin: string,
	circleId: string,
	payload: ReactionQueuePayload
): Promise<number> {
	return enqueue('reaction', origin, circleId, payload);
}

export async function enqueueReactionRemove(
	origin: string,
	circleId: string,
	postId: string
): Promise<number> {
	return enqueue('reaction_remove', origin, circleId, { post_id: postId });
}

export async function updateQueuedPost(
	id: number,
	payload: PostQueuePayload,
	files?: QueueFile[]
): Promise<void> {
	const item = await getQueueItem(id);
	if (!item || item.type !== 'post') return;
	if (item.state === 'uploading') return;

	await putQueueItem(id, {
		...item,
		payload,
		files: files ?? item.files,
		state: 'pending',
		error: undefined,
		uploads: undefined
	});
	notify();
	if (navigator.onLine) void drainQueue();
}

export async function removeQueueItem(id: number): Promise<void> {
	const item = await getQueueItem(id);
	if (!item || item.state === 'uploading') return;
	await deleteQueueItem(id);
	notify();
}

export async function retryQueueItem(id: number): Promise<void> {
	const item = await getQueueItem(id);
	if (!item || item.state !== 'failed') return;

	await putQueueItem(id, { ...item, state: 'pending', error: undefined });
	notify();
	if (navigator.onLine) void drainQueue();
}

export async function loadQueuedPost(id: number): Promise<QueueRecordWithId | undefined> {
	const item = await getQueueItem(id);
	if (!item || item.type !== 'post') return undefined;
	return { ...item, id };
}

export async function listQueuedPosts(
	origin: string,
	circleId: string
): Promise<QueuedPostView[]> {
	const items = await listQueueForCircle(origin, circleId);
	return items
		.filter((item) => item.type === 'post')
		.map((item) => {
			const payload = item.payload as PostQueuePayload;
			return {
				id: item.id,
				body: payload.body,
				entry_date: payload.entry_date,
				file_count: item.files.length,
				state: item.state,
				error: item.error
			};
		});
}

export async function listQueuedComments(
	origin: string,
	circleId: string,
	postId?: string
): Promise<QueuedCommentView[]> {
	const items = await listQueueForCircle(origin, circleId);
	return items
		.filter((item) => item.type === 'comment')
		.filter((item) => !postId || (item.payload as CommentQueuePayload).post_id === postId)
		.map((item) => {
			const payload = item.payload as CommentQueuePayload;
			return {
				id: item.id,
				post_id: payload.post_id,
				body: payload.body,
				state: item.state,
				error: item.error
			};
		});
}

export function getFailedItems(items: QueueRecordWithId[]): QueueRecordWithId[] {
	return items.filter((item) => item.state === 'failed' && item.error);
}

export { listQueueForCircle, listQueueItems, type QueueRecordWithId };
