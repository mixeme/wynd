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

const RETRY_MS = 5000;

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
			const { done, value } = await reader.read();
			if (done) break;
			buffer += decoder.decode(value, { stream: true });

			let parsed = takeSSEDataEvents(buffer);
			buffer = parsed.rest;
			for (const data of parsed.events) {
				await onData(data);
			}
		}
	} finally {
		reader.releaseLock();
	}
}

/** Splits a buffer on SSE blank-line delimiters and extracts `data:` payloads. */
export function takeSSEDataEvents(buffer: string): { events: string[]; rest: string } {
	const events: string[] = [];
	let rest = buffer;
	let split = rest.indexOf('\n\n');
	while (split !== -1) {
		const block = rest.slice(0, split);
		rest = rest.slice(split + 2);
		const dataLine = block.split('\n').find((line) => line.startsWith('data: '));
		if (dataLine) {
			events.push(dataLine.slice(6));
		}
		split = rest.indexOf('\n\n');
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

async function handleSyncEvent(origin: string, raw: string): Promise<void> {
	let event: SyncEvent;
	try {
		event = JSON.parse(raw) as SyncEvent;
	} catch {
		return;
	}
	await putCursor(origin, event.seq);
	await invalidateSnapshots(origin, { kind: 'circles' });
	await invalidateCircleSnapshots(origin, event.circle_id);
	triggerRefetch(origin, event.circle_id);
}

async function runSyncLoop(origin: string, signal: AbortSignal): Promise<void> {
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
			await sleep(RETRY_MS, signal);
			continue;
		}

		if (!res.body) {
			await sleep(RETRY_MS, signal);
			continue;
		}

		void drainQueue();

		try {
			await readSSE(res.body, signal, (data) => handleSyncEvent(origin, data));
		} catch (err) {
			if (signal.aborted) return;
		}

		await sleep(RETRY_MS, signal);
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
