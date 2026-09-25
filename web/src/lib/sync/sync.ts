import { ApiError, apiFetch, normalizeOrigin } from '$lib/api/client';
import { dropParticipantSession, isSessionRejected } from '$lib/session/session.svelte';
import {
	invalidateCircleSnapshots,
	invalidateSnapshots,
	type SnapshotKind
} from '$lib/api/snapshots';
import { getCursor, getSession, listSessions, putCursor } from '$lib/idb/db';
import { drainQueue } from '$lib/queue/queue';

export interface SyncEvent {
	seq: number;
	id: string;
	circle_id: string;
	type: string;
	is_service: boolean;
	actor_name: string;
	summary: string;
	created_at: string;
	payload: unknown;
	actor_identity_id?: string;
	target_id?: string;
}

export interface RefetchRegistration {
	origin: string;
	circleId?: string;
	kinds?: SnapshotKind[];
	refetch: () => void | Promise<void>;
}

const streams = new Map<string, AbortController>();
const refetchRegistrations: RefetchRegistration[] = [];

const RETRY_BASE_MS = 5000;
const RETRY_MAX_MS = 60000;
/**
 * Сторож тишины: сервер шлёт комментарный кадр раз в 15 с, поэтому пауза
 * длиннее 45 с означает мёртвое соединение. Без него тихо умерший TCP оставлял
 * клиента «подключённым», и приложение не обновлялось до перезагрузки (API-3).
 */
const WATCHDOG_MS = 45000;

/** Пауза перед повтором: min(5 с · 2^n, 60 с) с джиттером ±20 %. */
export function retryDelayMs(attempt: number, random: () => number = Math.random): number {
	const capped = Math.min(RETRY_BASE_MS * 2 ** Math.max(0, attempt), RETRY_MAX_MS);
	const jitter = 1 + (random() * 0.4 - 0.2);
	return Math.round(capped * jitter);
}

function sleep(ms: number, signal: AbortSignal): Promise<void> {
	return new Promise((resolve, reject) => {
		if (signal.aborted) {
			reject(signal.reason);
			return;
		}
		const timer = setTimeout(resolve, ms);
		signal.addEventListener(
			'abort',
			() => {
				clearTimeout(timer);
				reject(signal.reason);
			},
			{ once: true }
		);
	});
}

async function readSSE(
	body: ReadableStream<Uint8Array>,
	signal: AbortSignal,
	onData: (data: string) => void | Promise<void>
): Promise<void> {
	const reader = body.getReader();
	const decoder = new TextDecoder();
	let buffer = '';

	try {
		while (!signal.aborted) {
			// read() без таймаута ждёт вечно, если TCP умер молча, поэтому
			// чтение и сторож тишины идут наперегонки.
			let watchdog: ReturnType<typeof setTimeout> | undefined;
			const silence = new Promise<'silence'>((resolve) => {
				watchdog = setTimeout(() => resolve('silence'), WATCHDOG_MS);
			});
			let chunk: ReadableStreamReadResult<Uint8Array> | 'silence';
			try {
				chunk = await Promise.race([reader.read(), silence]);
			} finally {
				clearTimeout(watchdog);
			}
			if (chunk === 'silence') {
				throw new Error('sync: поток молчит дольше сторожа');
			}
			const { done, value } = chunk;
			if (done) break;
			buffer += decoder.decode(value, { stream: true });

			const parsed = takeSSEDataEvents(buffer);
			buffer = parsed.rest;
			for (const data of parsed.events) {
				await onData(data);
			}
		}
	} finally {
		reader.releaseLock();
	}
}

/**
 * Разбор потока по спецификации SSE: разделитель кадров — пустая строка в любом
 * из видов (LF, CRLF, CR), пробел после «data:» необязателен, несколько строк
 * `data` склеиваются переводом строки. Комментарные кадры (`:`) и блоки без
 * `data` событиями не считаются (API-3).
 */
export function takeSSEDataEvents(buffer: string): { events: string[]; rest: string } {
	const events: string[] = [];
	const separator = /\r\n\r\n|\n\n|\r\r/;
	let rest = buffer;
	for (;;) {
		const match = separator.exec(rest);
		if (!match) break;
		const block = rest.slice(0, match.index);
		rest = rest.slice(match.index + match[0].length);
		const parts: string[] = [];
		for (const line of block.split(/\r\n|\n|\r/)) {
			if (!line.startsWith('data:')) continue;
			const value = line.slice(5);
			parts.push(value.startsWith(' ') ? value.slice(1) : value);
		}
		if (parts.length > 0) events.push(parts.join('\n'));
	}
	return { events, rest };
}

function triggerRefetch(origin: string, circleId: string): void {
	const normalized = normalizeOrigin(origin);
	for (const reg of refetchRegistrations) {
		if (normalizeOrigin(reg.origin) !== normalized) continue;
		if (reg.circleId && reg.circleId !== circleId) continue;
		void reg.refetch();
	}
}

function triggerRefetchAll(origin: string): void {
	const normalized = normalizeOrigin(origin);
	for (const reg of refetchRegistrations) {
		if (normalizeOrigin(reg.origin) === normalized) void reg.refetch();
	}
}

/**
 * Первый кадр потока — `{"max_seq": N}`. Курсор больше N значит, что сервер
 * восстановлен из бэкапа и наши номера событий из прежней жизни: сервер уже
 * считает всё до N увиденным, а здесь сбрасываются курсор и снимки, чтобы
 * экраны перечитали состояние (план 42, BKP-3).
 */
async function handleHello(origin: string, maxSeq: number): Promise<void> {
	const cursor = (await getCursor(origin))?.seq ?? 0;
	if (cursor <= maxSeq) return;
	await putCursor(origin, maxSeq);
	await invalidateSnapshots(origin);
	triggerRefetchAll(origin);
}

/** Разбирает кадр `data:` потока sync: событие хроники или приветствие `max_seq`. */
export async function handleSyncFrame(origin: string, raw: string): Promise<void> {
	let event: Partial<SyncEvent> & { max_seq?: number };
	try {
		event = JSON.parse(raw) as typeof event;
	} catch {
		return;
	}
	if (typeof event.seq !== 'number') {
		if (typeof event.max_seq === 'number') await handleHello(origin, event.max_seq);
		return;
	}
	await putCursor(origin, event.seq);
	await invalidateSnapshots(origin, { kind: 'circles' });
	await invalidateCircleSnapshots(origin, event.circle_id ?? '');
	triggerRefetch(origin, event.circle_id ?? '');
}

async function runSyncLoop(origin: string, signal: AbortSignal): Promise<void> {
	let attempt = 0;
	while (!signal.aborted) {
		const session = await getSession(origin);
		if (!session?.token) {
			stopSync(origin);
			return;
		}

		const cursor = (await getCursor(origin))?.seq ?? 0;

		let res: Response;
		try {
			res = await apiFetch(origin, `/sync?cursor=${cursor}`, {
				headers: { Accept: 'text/event-stream' },
				signal
			});
		} catch (err) {
			if (signal.aborted) return;
			if (isSessionRejected(err)) {
				stopSync(origin);
				await dropParticipantSession(origin);
				return;
			}
			await sleep(retryDelayMs(attempt++), signal);
			continue;
		}

		if (!res.body) {
			await sleep(retryDelayMs(attempt++), signal);
			continue;
		}

		// Поток открыт — счётчик неудач сброшен.
		attempt = 0;
		void drainQueue();

		try {
			await readSSE(res.body, signal, (data) => handleSyncFrame(origin, data));
		} catch {
			if (signal.aborted) return;
			attempt++;
		}

		await sleep(retryDelayMs(attempt), signal);
	}
}

export function startSync(origin: string): void {
	const key = normalizeOrigin(origin);
	if (streams.has(key)) return;

	const ac = new AbortController();
	streams.set(key, ac);
	void runSyncLoop(origin, ac.signal).finally(() => {
		if (streams.get(key) === ac) {
			streams.delete(key);
		}
	});
}

export function stopSync(origin: string): void {
	const key = normalizeOrigin(origin);
	streams.get(key)?.abort();
	streams.delete(key);
}

export async function startSyncForAllSessions(): Promise<void> {
	const sessions = await listSessions();
	for (const session of sessions) {
		startSync(session.origin);
	}
}

export function registerRefetch(reg: RefetchRegistration): () => void {
	refetchRegistrations.push(reg);
	return () => {
		const index = refetchRegistrations.indexOf(reg);
		if (index >= 0) refetchRegistrations.splice(index, 1);
	};
}
