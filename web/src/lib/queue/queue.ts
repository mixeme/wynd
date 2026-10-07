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
import { sha256Hex } from '$lib/sha256';
import { holdWakeLock } from '$lib/media/wake-lock';
import { uuid } from '$lib/uuid';

export const CHUNK_SIZE = 1024 * 1024;

/** Вложение записи в очереди — чтобы лента показала его тем, что оно есть. */
export interface QueuedMediaView {
	kind: 'photo' | 'video' | 'attachment';
	name: string;
	size: number;
	voice?: boolean;
	duration_ms?: number;
	peaks?: number[];
	/** Картинка для плитки: сам снимок или кадр ролика. */
	preview?: { type: string; data: ArrayBuffer };
}

export interface QueuedPostView {
	id: number;
	body: string;
	entry_date: string;
	file_count: number;
	media: QueuedMediaView[];
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

/** Зависшая после падения вкладки отправка снова считается ожидающей (QUE-1). */
const UPLOADING_STALE_MS = 2 * 60 * 1000;

const RETRY_BASE_MS = 30 * 1000;
const RETRY_MAX_MS = 15 * 60 * 1000;

/** Пауза перед повтором отправки: min(30 с · 2^n, 15 мин). */
export function queueRetryDelayMs(attempts: number): number {
	return Math.min(RETRY_BASE_MS * 2 ** Math.max(0, attempts - 1), RETRY_MAX_MS);
}

/**
 * Что можно отправлять прямо сейчас: ожидающие и зависшие в uploading, чья
 * пауза после неудачи уже вышла.
 */
export function readyQueueItems(
	items: QueueRecordWithId[],
	now: number = Date.now()
): QueueRecordWithId[] {
	return items
		.filter((item) => {
			if (item.state === 'failed') return false;
			if (item.state === 'uploading') {
				const since = item.uploading_at ?? 0;
				if (now - since < UPLOADING_STALE_MS) return false;
			}
			const attempts = item.attempts ?? 0;
			if (attempts > 0) {
				const last = item.uploading_at ?? item.created_at;
				if (now - last < queueRetryDelayMs(attempts)) return false;
			}
			return true;
		})
		.sort((a, b) => a.created_at - b.created_at);
}

function notify(): void {
	for (const listener of listeners) {
		listener();
	}
}

export function subscribeQueue(listener: QueueListener): () => void {
	listeners.add(listener);
	return () => listeners.delete(listener);
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
					// Без имени сервер сохранял «.»: вложение из очереди скачивалось
					// безымянным (найдено на голосовых, C14).
					body: JSON.stringify({
						expected_size: files[i].size,
						mime_type: files[i].type,
						filename: files[i].name
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

		const mediaMeta = [...(postPayload.media_meta ?? [])];
		const media = [];
		for (let i = 0; i < blobIds.length; i++) {
			let meta: QueueMediaMeta = mediaMeta[i] ?? { kind: 'photo' };
			let coverId = meta.audio_cover_blob_id;
			if (!coverId && meta.audio_cover) {
				coverId = await uploadFile(origin, {
					name: 'cover.jpg',
					type: 'image/jpeg',
					size: meta.audio_cover.data.byteLength,
					data: meta.audio_cover.data
				});
				const saved: QueueMediaMeta = { ...meta, audio_cover_blob_id: coverId };
				delete saved.audio_cover;
				meta = saved;
				mediaMeta[i] = saved;
				const nextPayload: PostQueuePayload = { ...postPayload, media_meta: mediaMeta };
				await putQueueItem(item.id, { ...item, payload: nextPayload, state: 'uploading', uploads });
			}
			let posterId = meta.video_poster_blob_id;
			if (!posterId && meta.video_poster) {
				posterId = await uploadFile(origin, {
					name: 'poster.jpg',
					type: 'image/jpeg',
					size: meta.video_poster.data.byteLength,
					data: meta.video_poster.data
				});
				const saved: QueueMediaMeta = { ...meta, video_poster_blob_id: posterId };
				delete saved.video_poster;
				meta = saved;
				mediaMeta[i] = saved;
				const nextPayload: PostQueuePayload = { ...postPayload, media_meta: mediaMeta };
				await putQueueItem(item.id, { ...item, payload: nextPayload, state: 'uploading', uploads });
			}
			media.push({
				blob_id: blobIds[i],
				kind: meta.kind,
				captured_at: meta.captured_at,
				geo_lat: meta.geo_lat,
				geo_lng: meta.geo_lng,
				is_cover: meta.is_cover ?? false,
				audio_artist: meta.audio_artist,
				audio_title: meta.audio_title,
				audio_cover_blob_id: coverId,
				video_poster_blob_id: posterId,
				voice: meta.voice,
				audio_duration_ms: meta.audio_duration_ms,
				audio_peaks: meta.audio_peaks,
				crop: meta.is_cover ? meta.crop : undefined
			});
		}

		await apiJson(origin, `/circles/${circleId}/posts`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({
				body: postPayload.body,
				entry_date: postPayload.entry_date,
				captured_at: postPayload.captured_at,
				client_id: item.client_id,
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
				body: JSON.stringify({ body: commentPayload.body, client_id: item.client_id })
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
	await putQueueItem(item.id, {
		...item,
		state: 'uploading',
		error: undefined,
		uploading_at: Date.now()
	});
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
		const attempts = (latest.attempts ?? 0) + 1;
		// 401 — не приговор отправке: сессия могла истечь, и после входа
		// запись должна уйти. Остальные 4xx — отказ по существу (QUE-1).
		if (err instanceof ApiError && err.status >= 400 && err.status < 500 && err.status !== 401) {
			await putQueueItem(item.id, {
				...latest,
				state: 'failed',
				attempts,
				uploading_at: Date.now(),
				error: err.code
			});
			notify();
			return false;
		}

		await putQueueItem(item.id, {
			...latest,
			state: 'pending',
			attempts,
			uploading_at: Date.now()
		});
		notify();
		throw err;
	}
}

async function drainOnce(): Promise<void> {
	// Экран не гаснет, пока очередь уходит: загрузка видео — минуты (A5).
	let releaseWake: (() => void) | undefined;
	try {
		await drainLoop(() => {
			releaseWake ??= holdWakeLock();
		});
	} finally {
		releaseWake?.();
	}
}

async function drainLoop(onWork: () => void): Promise<void> {
	while (navigator.onLine) {
		const ready = readyQueueItems(await listQueueItems());
		if (!ready.length) break;
		onWork();
		// Недоступный сервер задерживает только свою очередь: раньше общий
		// FIFO с break останавливал отправку и на живые серверы (QUE-1).
		const blocked = new Set<string>();
		let progressed = false;
		for (const item of ready) {
			if (blocked.has(item.origin)) continue;
			try {
				await processItem(item);
				progressed = true;
			} catch {
				blocked.add(item.origin);
			}
		}
		if (!progressed) break;
	}
}

export async function drainQueue(): Promise<void> {
	if (draining) return draining;

	// Между вкладками слив держит Web Locks: флаг в модуле — на вкладку, и
	// две вкладки сливали одну очередь дважды (QUE-1).
	const run = async () => {
		const locks = typeof navigator !== 'undefined' ? navigator.locks : undefined;
		if (!locks) {
			await drainOnce();
			return;
		}
		await locks.request('wynd-queue', { ifAvailable: true }, async (lock) => {
			if (!lock) return;
			await drainOnce();
		});
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
		// Ключ идемпотентности ставится один раз при постановке: сколько бы
		// раз отправка ни повторилась, сервер создаст одну сущность (CLI-2).
		client_id: uuid(),
		payload,
		files,
		state: 'pending',
		attempts: 0,
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

/** Что лежит в записи очереди: по файлу и его описанию, как их отправит очередь. */
export function queuedMediaViews(
	files: QueueFile[],
	metas: QueueMediaMeta[] | undefined
): QueuedMediaView[] {
	return files.map((file, i) => {
		const meta = metas?.[i];
		const kind = meta?.kind === 'video' || meta?.kind === 'attachment' ? meta.kind : 'photo';
		const view: QueuedMediaView = { kind, name: file.name, size: file.size };
		if (kind === 'photo') view.preview = { type: file.type, data: file.data };
		else if (kind === 'video' && meta?.video_poster) view.preview = meta.video_poster;
		if (meta?.voice) {
			view.voice = true;
			view.duration_ms = meta.audio_duration_ms;
			view.peaks = meta.audio_peaks;
		}
		return view;
	});
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
				media: queuedMediaViews(item.files, payload.media_meta),
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
