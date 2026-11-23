import { ApiError, apiFetch, apiJson, normalizeOrigin } from '$lib/api/client';
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
import { createSha256 } from '$lib/sha256';
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
	/**
	 * Ролик, кадр которого снять не удалось: Firefox на Android не отдаёт
	 * кадр файла в canvas. Плитка показывает сам ролик на паузе (C107).
	 */
	video?: { type: string; data: ArrayBuffer };
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
	media: QueuedMediaView[];
	state: QueueRecord['state'];
	error?: string;
}

type QueueListener = () => void;

const listeners = new Set<QueueListener>();
let draining: Promise<void> | undefined;
let activeDrain: AbortController | undefined;

/** Запасной срок для клиентов без Web Locks (QUE-1). */
const UPLOADING_STALE_MS = 2 * 60 * 1000;

const RETRY_BASE_MS = 30 * 1000;
const RETRY_MAX_MS = 15 * 60 * 1000;

/** Пауза перед повтором отправки: min(30 с · 2^n, 15 мин). */
export function queueRetryDelayMs(attempts: number): number {
	return Math.min(RETRY_BASE_MS * 2 ** Math.max(0, attempts - 1), RETRY_MAX_MS);
}

/**
 * Что можно отправлять сейчас с учётом паузы после неудачи и блокировки очереди.
 */
export function readyQueueItems(
	items: QueueRecordWithId[],
	now: number = Date.now(),
	ownsQueue = false
): QueueRecordWithId[] {
	return items
		.filter((item) => {
			if (item.state === 'failed') return false;
			if (item.state === 'uploading') {
				// Под эксклюзивной блокировкой отправку оставила закрытая вкладка.
				if (ownsQueue) return true;
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

type PostSentListener = (origin: string, circleId: string) => Promise<void>;
const postSentListeners = new Set<PostSentListener>();
const POST_SENT_WAIT_MS = 5000;

/**
 * Запись принята сервером: экран круга перечитывает ленту, пока карточка
 * очереди ещё стоит. Ждём не дольше POST_SENT_WAIT_MS — медленная лента не
 * держит очередь.
 */
export function subscribePostSent(listener: PostSentListener): () => void {
	postSentListeners.add(listener);
	return () => postSentListeners.delete(listener);
}

async function announcePostSent(origin: string, circleId: string): Promise<void> {
	if (!postSentListeners.size) return;
	const all = Promise.all(
		[...postSentListeners].map((l) => l(origin, circleId).catch(() => undefined))
	);
	await Promise.race([all, new Promise((resolve) => setTimeout(resolve, POST_SENT_WAIT_MS))]);
}

/**
 * Ход отправки записи: доля ушедших байт, 0..1. Живёт в памяти вкладки и идёт
 * мимо `notify`: тот перечитывает очередь из IndexedDB вместе с файлами, а
 * на каждый мегабайт ролика это слишком дорого.
 */
type QueueProgressListener = (id: number, fraction: number | undefined) => void;

const progressListeners = new Set<QueueProgressListener>();
const sendProgress = new Map<number, number>();

function setSendProgress(id: number, fraction: number | undefined): void {
	if (fraction === undefined) {
		if (!sendProgress.delete(id)) return;
	} else {
		sendProgress.set(id, fraction);
	}
	for (const listener of progressListeners) {
		listener(id, fraction);
	}
}

/** Подписка на ход отправки; уже идущие отправки приходят сразу. */
export function subscribeQueueProgress(listener: QueueProgressListener): () => void {
	progressListeners.add(listener);
	for (const [id, fraction] of sendProgress) listener(id, fraction);
	return () => progressListeners.delete(listener);
}

/**
 * Подпись записи в очереди: «в очереди», пока ждёт, и «отправляется», пока
 * уходит. Процент есть, когда файлы больше одной порции: мелкое уходит разом,
 * и «0%» у него только обманывал бы.
 */
export function queuedTimeLabel(
	state: QueueRecord['state'],
	fraction?: number,
	error?: string
): string {
	// Места нет (C42, 3.12): запись ждёт и уйдёт сама, когда место появится.
	if (state !== 'uploading' && error === QUOTA_CIRCLE) return 'в очереди · в круге кончилось место';
	if (state !== 'uploading' && error === QUOTA_SERVER)
		return 'в очереди · на сервере кончилось место';
	if (state !== 'uploading') return 'в очереди';
	if (fraction === undefined) return 'отправляется';
	return `отправляется · ${Math.min(100, Math.floor(fraction * 100))}%`;
}

async function getUploadOffset(
	origin: string,
	sessionId: string,
	signal: AbortSignal
): Promise<number> {
	const res = await apiFetch(origin, `/uploads/${sessionId}`, { method: 'HEAD', signal });
	const offset = res.headers.get('Upload-Offset');
	return offset ? parseInt(offset, 10) : 0;
}

async function uploadFile(
	origin: string,
	file: QueueFile,
	signal: AbortSignal,
	progress?: QueueUploadProgress,
	onSent?: (bytes: number) => void
): Promise<string> {
	let sessionId = progress?.session_id;
	let offset = 0;
	const bytes = file.data instanceof Uint8Array ? file.data : new Uint8Array(file.data);
	const hash = createSha256();

	if (sessionId) {
		offset = await getUploadOffset(origin, sessionId, signal);
		onSent?.(offset);
		// Уже принятая часть входит в SHA, даже если файл догружается.
		if (offset > 0) hash.update(bytes.subarray(0, offset));
	} else {
		const session = await apiJson<{ id: string }>(origin, '/uploads', {
			method: 'POST',
			signal,
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ expected_size: file.size, mime_type: file.type, filename: file.name })
		});
		sessionId = session.id;
	}

	while (offset < bytes.byteLength) {
		const end = Math.min(offset + CHUNK_SIZE, bytes.byteLength);
		const chunk = bytes.slice(offset, end);
		// SHA считаем по тем же байтам, что уходят, и до отправки (C119):
		// один крупный `crypto.subtle.digest` после загрузки дал неверный SHA.
		hash.update(chunk);
		const res = await apiFetch(origin, `/uploads/${sessionId}`, {
			method: 'PUT',
			signal,
			headers: {
				'Content-Type': 'application/octet-stream',
				'Upload-Offset': String(offset)
			},
			body: chunk
		});
		const body = (await res.json()) as { received_bytes: number };
		offset = body.received_bytes;
		onSent?.(offset);
	}

	const complete = await apiJson<{ id: string }>(origin, `/uploads/${sessionId}/complete`, {
		method: 'POST',
		signal,
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ sha256: hash.hex() })
	});

	return complete.id;
}

/**
 * Грузит файлы записи очереди по одному и запоминает, что уже ушло: после
 * обрыва отправка продолжается с того же места, а не с начала.
 */
async function uploadItemFiles(
	item: QueueRecordWithId,
	signal: AbortSignal
): Promise<{ blobIds: string[]; uploads: QueueUploadProgress[] }> {
	const { origin, files } = item;
	const blobIds: string[] = [];
	let uploads = [...(item.uploads ?? [])];
	const total = files.reduce((sum, file) => sum + file.size, 0);
	let done = 0;
	const report = (sent: number) => {
		if (total > CHUNK_SIZE) setSendProgress(item.id, Math.min(1, (done + sent) / total));
	};

	for (let i = 0; i < files.length; i++) {
		let progress = uploads.find((u) => u.file_index === i);
		if (progress?.blob_id) {
			blobIds.push(progress.blob_id);
			done += files[i].size;
			report(0);
			continue;
		}
		for (let attempt = 0; attempt < 2; attempt++) {
			if (!progress) {
				const session = await apiJson<{ id: string }>(origin, '/uploads', {
					method: 'POST',
					signal,
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
			try {
				const blobId = await uploadFile(origin, files[i], signal, progress, report);
				done += files[i].size;
				progress = { ...progress, blob_id: blobId };
				uploads = [...uploads.filter((u) => u.file_index !== i), progress];
				await putQueueItem(item.id, { ...item, state: 'uploading', uploads });
				blobIds.push(blobId);
				break;
			} catch (err) {
				if (!(err instanceof ApiError) || err.status !== 404) throw err;
				// Only upload endpoints are retried here. Keep the original and other
				// finished files, and persist the reset even if creating a session fails.
				progress = undefined;
				uploads = uploads.filter((u) => u.file_index !== i);
				await putQueueItem(item.id, { ...item, state: 'uploading', uploads });
				if (attempt === 1) throw err;
			}
		}
	}
	return { blobIds, uploads };
}

async function submitQueueItem(item: QueueRecordWithId, signal: AbortSignal): Promise<void> {
	const { origin, circle_id: circleId, type, payload } = item;

	if (type === 'post') {
		const postPayload = payload as PostQueuePayload;
		const { blobIds, uploads } = await uploadItemFiles(item, signal);

		const mediaMeta = [...(postPayload.media_meta ?? [])];
		const media = [];
		for (let i = 0; i < blobIds.length; i++) {
			let meta: QueueMediaMeta = mediaMeta[i] ?? { kind: 'photo' };
			let coverId = meta.audio_cover_blob_id;
			if (!coverId && meta.audio_cover) {
				coverId = await uploadFile(
					origin,
					{
						name: 'cover.jpg',
						type: 'image/jpeg',
						size: meta.audio_cover.data.byteLength,
						data: meta.audio_cover.data
					},
					signal
				);
				const saved: QueueMediaMeta = { ...meta, audio_cover_blob_id: coverId };
				delete saved.audio_cover;
				meta = saved;
				mediaMeta[i] = saved;
				const nextPayload: PostQueuePayload = { ...postPayload, media_meta: mediaMeta };
				await putQueueItem(item.id, { ...item, payload: nextPayload, state: 'uploading', uploads });
			}
			let posterId = meta.video_poster_blob_id;
			if (!posterId && meta.video_poster) {
				posterId = await uploadFile(
					origin,
					{
						name: 'poster.jpg',
						type: 'image/jpeg',
						size: meta.video_poster.data.byteLength,
						data: meta.video_poster.data
					},
					signal
				);
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
				video_duration_ms: meta.kind === 'video' ? meta.video_duration_ms : undefined,
				voice: meta.voice,
				audio_duration_ms: meta.audio_duration_ms,
				audio_peaks: meta.audio_peaks,
				crop: meta.is_cover ? meta.crop : undefined
			});
		}

		await apiJson(origin, `/circles/${circleId}/posts`, {
			method: 'POST',
			signal,
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
		const { blobIds } = await uploadItemFiles(item, signal);
		const media = blobIds.map((blobId, i) => {
			const meta = commentPayload.media_meta?.[i];
			return {
				blob_id: blobId,
				kind: meta?.kind === 'attachment' ? 'attachment' : 'photo',
				voice: meta?.voice,
				audio_duration_ms: meta?.audio_duration_ms,
				audio_peaks: meta?.audio_peaks
			};
		});
		await apiJson(origin, `/circles/${circleId}/posts/${commentPayload.post_id}/comments`, {
			method: 'POST',
			signal,
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ body: commentPayload.body, client_id: item.client_id, media })
		});
		return;
	}

	if (type === 'reaction') {
		const reactionPayload = payload as ReactionQueuePayload;
		await apiJson(origin, `/circles/${circleId}/posts/${reactionPayload.post_id}/reactions`, {
			method: 'PUT',
			signal,
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ emoji: reactionPayload.emoji })
		});
		return;
	}

	if (type === 'reaction_remove') {
		const reactionPayload = payload as ReactionQueuePayload;
		await apiFetch(origin, `/circles/${circleId}/posts/${reactionPayload.post_id}/reactions`, {
			method: 'DELETE',
			signal
		});
	}
}

const QUOTA_CIRCLE = 'quota_exceeded';
const QUOTA_SERVER = 'quota_exceeded:server';

async function processItem(item: QueueRecordWithId, signal: AbortSignal): Promise<boolean> {
	const uploading: QueueRecordWithId = {
		...item,
		state: 'uploading',
		error: undefined,
		uploading_at: Date.now()
	};
	await putQueueItem(item.id, uploading);
	notify();

	try {
		await submitQueueItem(uploading, signal);
		await invalidateCircleSnapshots(item.origin, item.circle_id);
		// Запись уходит из очереди, когда лента её уже показывает: раньше
		// карточка очереди пропадала на 100%, а запись появлялась только с
		// обновлением ленты (владелец, 2026-10-22).
		if (item.type === 'post') await announcePostSent(item.origin, item.circle_id);
		await deleteQueueItem(item.id);
		setSendProgress(item.id, undefined);
		notify();
		return true;
	} catch (err) {
		setSendProgress(item.id, undefined);
		const stored = await getQueueItem(item.id);
		const latest = stored ? { ...stored, id: item.id } : item;
		const attempts = (latest.attempts ?? 0) + 1;
		// Места нет (507, C42) — не отказ: запись ждёт с подписью и уходит
		// сама, когда место появится. Остальные записи сервера не держит.
		if (err instanceof ApiError && err.status === 507) {
			await putQueueItem(item.id, {
				...latest,
				state: 'pending',
				attempts,
				uploading_at: Date.now(),
				error: err.detail === 'server' ? QUOTA_SERVER : QUOTA_CIRCLE
			});
			notify();
			return false;
		}
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
			attempts: signal.aborted && !(err instanceof ApiError) ? 0 : attempts,
			uploading_at: Date.now()
		});
		notify();
		throw err;
	}
}

async function drainOnce(ownsQueue: boolean): Promise<void> {
	// Экран не гаснет, пока очередь уходит: загрузка видео — минуты (A5).
	let releaseWake: (() => void) | undefined;
	try {
		const onWork = () => {
			releaseWake ??= holdWakeLock();
		};
		do {
			activeDrain = new AbortController();
			await drainLoop(onWork, activeDrain.signal, ownsQueue);
			// Связь могла вернуться, пока отменённая отправка сохраняла прогресс.
		} while (activeDrain.signal.aborted && navigator.onLine);
	} finally {
		activeDrain = undefined;
		releaseWake?.();
	}
	void scheduleNextDrain();
}

// Повтор после неудачи не должен ждать внешнего события (вход, «online»,
// переподключение потока): окно живёт в трее, и очередь обязана дойти сама.
let retryTimer: ReturnType<typeof setTimeout> | undefined;

export function nextRetryDelay(items: QueueRecordWithId[], now: number): number | undefined {
	let delay: number | undefined;
	for (const item of items) {
		if (item.state === 'failed') continue;
		let candidate: number;
		if (item.state === 'uploading') {
			candidate = UPLOADING_STALE_MS - (now - (item.uploading_at ?? 0));
		} else {
			const attempts = item.attempts ?? 0;
			if (attempts === 0 && !item.uploading_at) continue;
			// attempts=0 с uploading_at — отправку прервали (переход, отмена):
			// повторяем как после первой неудачи, иначе запись ждёт события.
			candidate =
				queueRetryDelayMs(Math.max(1, attempts)) - (now - (item.uploading_at ?? item.created_at));
		}
		if (delay === undefined || candidate < delay) delay = candidate;
	}
	return delay;
}

async function scheduleNextDrain(): Promise<void> {
	if (retryTimer) return;
	const delay = nextRetryDelay(await listQueueItems(), Date.now());
	if (delay === undefined) return;
	retryTimer = setTimeout(
		() => {
			retryTimer = undefined;
			if (navigator.onLine) void drainQueue();
			else void scheduleNextDrain();
		},
		Math.max(1000, delay)
	);
}

async function drainLoop(
	onWork: () => void,
	signal: AbortSignal,
	ownsQueue: boolean
): Promise<void> {
	while (!signal.aborted && navigator.onLine) {
		const ready = readyQueueItems(await listQueueItems(), Date.now(), ownsQueue);
		if (!ready.length) break;
		onWork();
		// Недоступный сервер задерживает только свою очередь: раньше общий
		// FIFO с break останавливал отправку и на живые серверы (QUE-1).
		const blocked = new Set<string>();
		let progressed = false;
		for (const item of ready) {
			if (signal.aborted || !navigator.onLine) return;
			if (blocked.has(item.origin)) continue;
			try {
				await processItem(item, signal);
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
			await drainOnce(false);
			return;
		}
		await locks.request('wynd-queue', { ifAvailable: true }, async (lock) => {
			if (!lock) return;
			await drainOnce(true);
		});
	};

	const promise = run().finally(() => {
		if (draining === promise) draining = undefined;
	});
	draining = promise;
	return promise;
}

/**
 * Сервер снова отвечает: его ожидающие записи уходят сразу, не дожидаясь
 * паузы после прежних неудач — пауза берегла молчавший сервер (C25).
 */
export async function drainQueueFor(origin: string): Promise<void> {
	const key = normalizeOrigin(origin);
	let woke = false;
	for (const item of await listQueueItems()) {
		if (normalizeOrigin(item.origin) !== key) continue;
		if (item.state !== 'pending' || !(item.attempts ?? 0)) continue;
		await putQueueItem(item.id, { ...item, attempts: 0 });
		woke = true;
	}
	if (woke) notify();
	return drainQueue();
}

export function initQueueDrain(): () => void {
	const onOnline = () => void drainQueue();
	const onOffline = () => activeDrain?.abort();
	window.addEventListener('online', onOnline);
	window.addEventListener('offline', onOffline);
	if (navigator.onLine) void drainQueue();
	return () => {
		window.removeEventListener('online', onOnline);
		window.removeEventListener('offline', onOffline);
	};
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
	payload: CommentQueuePayload,
	files: QueueFile[] = []
): Promise<number> {
	return enqueue('comment', origin, circleId, payload, files);
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

	await putQueueItem(id, { ...item, state: 'pending', attempts: 0, error: undefined });
	notify();
	if (navigator.onLine) void drainQueue();
}

export async function loadQueuedPost(id: number): Promise<QueueRecordWithId | undefined> {
	const item = await getQueueItem(id);
	if (!item || item.type !== 'post') return undefined;
	return { ...item, id };
}

/** Реплика в очереди целиком — для правки на месте (C53). */
export async function loadQueuedComment(id: number): Promise<QueueRecordWithId | undefined> {
	const item = await getQueueItem(id);
	if (!item || item.type !== 'comment') return undefined;
	return { ...item, id };
}

/**
 * Правка реплики, которая ещё в очереди (C53): слова и вложения заменяются
 * целиком, как у записи в очереди. Уже уходящую не трогаем.
 */
export async function updateQueuedComment(
	id: number,
	payload: CommentQueuePayload,
	files: QueueFile[]
): Promise<void> {
	const item = await getQueueItem(id);
	if (!item || item.type !== 'comment') return;
	if (item.state === 'uploading') return;

	await putQueueItem(id, {
		...item,
		payload,
		files,
		state: 'pending',
		error: undefined,
		uploads: undefined
	});
	notify();
	if (navigator.onLine) void drainQueue();
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
		else if (kind === 'video') {
			if (meta?.video_poster) view.preview = meta.video_poster;
			// Кадра нет — плитка покажет сам ролик: на Firefox Android только так
			// видно, что за ролик в очереди (C107).
			else view.video = { type: file.type || 'video/webm', data: file.data };
		}
		if (kind === 'video') view.duration_ms = meta?.video_duration_ms;
		if (meta?.voice) {
			view.voice = true;
			view.duration_ms = meta.audio_duration_ms;
			view.peaks = meta.audio_peaks;
		}
		return view;
	});
}

export async function listQueuedPosts(origin: string, circleId: string): Promise<QueuedPostView[]> {
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
				media: queuedMediaViews(item.files, payload.media_meta),
				state: item.state,
				error: item.error
			};
		});
}

export function getFailedItems(items: QueueRecordWithId[]): QueueRecordWithId[] {
	return items.filter((item) => item.state === 'failed' && item.error);
}

export { listQueueForCircle, listQueueItems, type QueueRecordWithId };
