/**
 * Нативная очередь (план 50, этап 5): записи уходят через WorkManager и после
 * закрытия окна. 5.1 — без вложений; 5.2 — файлы, постеры и обложки:
 * begin → chunk* → commit. Страница отдаёт запись и токен только главному
 * фрейму выбранного origin; native хранит их в private-каталоге.
 */
export interface NativeQueueInput {
	type: string;
	origin: string;
	circle_id: string;
	client_id: string;
	payload: unknown;
	token: string;
}

export interface NativeQueueStatus {
	client_id: string;
	native_id?: string;
	state: 'pending' | 'sent' | 'failed' | 'unauthorized';
	error?: string;
}

export interface NativeQueuePart {
	name: string;
	mime: string;
	size: number;
	media_index?: number;
	field?: 'video_poster' | 'audio_cover';
}

export interface NativeQueueBegin {
	type: string;
	origin: string;
	circle_id: string;
	client_id: string;
	payload: unknown;
	token: string;
	files: NativeQueuePart[];
	aux: NativeQueuePart[];
}

interface NativeQueuePort {
	postMessage(message: string): void;
	onmessage: ((event: { data: string }) => void) | null;
}

interface Reply {
	id: number;
	native_id?: string;
	token?: string;
	ok?: boolean;
	error?: string;
	records?: unknown;
	statuses?: unknown;
}

const TIMEOUT_MS = 5000;
let sequence = 0;
const connections = new WeakMap<
	NativeQueuePort,
	Map<
		number,
		{
			resolve: (reply: Reply) => void;
			reject: (error: Error) => void;
			timer: ReturnType<typeof setTimeout>;
		}
	>
>();

function port(): NativeQueuePort | undefined {
	if (typeof window === 'undefined') return undefined;
	const candidate = (window as unknown as { __wyndQueue?: NativeQueuePort }).__wyndQueue;
	return typeof candidate?.postMessage === 'function' ? candidate : undefined;
}

export function androidQueueAvailable(): boolean {
	return !!port();
}

function request(target: NativeQueuePort, value: Record<string, unknown>): Promise<Reply> {
	let pending = connections.get(target);
	if (!pending) {
		pending = new Map();
		connections.set(target, pending);
		const callbacks = pending;
		target.onmessage = (event) => {
			try {
				const reply: Reply = JSON.parse(event.data);
				if (!Number.isSafeInteger(reply.id)) return;
				const callback = callbacks.get(reply.id);
				if (!callback) return;
				callbacks.delete(reply.id);
				clearTimeout(callback.timer);
				if (typeof reply.error === 'string' && reply.error) {
					callback.reject(new Error(`Android queue: ${reply.error}`));
					return;
				}
				callback.resolve(reply);
			} catch {
				/* Повреждённый ответ не завершает запись. */
			}
		};
	}
	return new Promise((resolve, reject) => {
		const id = ++sequence;
		const timer = setTimeout(() => {
			pending.delete(id);
			reject(new Error('Android queue timed out'));
		}, TIMEOUT_MS);
		pending.set(id, { resolve, reject, timer });
		try {
			target.postMessage(JSON.stringify({ ...value, id }));
		} catch (error) {
			clearTimeout(timer);
			pending.delete(id);
			reject(error instanceof Error ? error : new Error('Android queue failed'));
		}
	});
}

function status(value: unknown): NativeQueueStatus | undefined {
	if (!value || typeof value !== 'object') return undefined;
	const item = value as NativeQueueStatus;
	if (typeof item.client_id !== 'string') return undefined;
	if (!['pending', 'sent', 'failed', 'unauthorized'].includes(item.state)) return undefined;
	const result: NativeQueueStatus = {
		client_id: item.client_id,
		state: item.state,
		error: typeof item.error === 'string' ? item.error : undefined
	};
	if (typeof item.native_id === 'string') result.native_id = item.native_id;
	return result;
}

function statuses(value: unknown): NativeQueueStatus[] {
	if (!Array.isArray(value)) return [];
	return value.map(status).filter((item): item is NativeQueueStatus => !!item);
}

/** Отдать запись native; undefined — моста нет, очередь отправит страница. */
export async function nativeEnqueue(input: NativeQueueInput): Promise<string | undefined> {
	const target = port();
	if (!target) return undefined;
	const reply = await request(target, { op: 'enqueue', ...input });
	if (typeof reply.native_id !== 'string' || !reply.native_id)
		throw new Error('Android queue: invalid reply');
	return reply.native_id;
}

/** Начать передачу файлов; дальше chunk* и commit. */
export async function nativeBegin(input: NativeQueueBegin): Promise<string> {
	const target = port();
	if (!target) throw new Error('Android queue unavailable');
	const reply = await request(target, { op: 'begin', ...input });
	if (typeof reply.token !== 'string' || !reply.token)
		throw new Error('Android queue: invalid begin');
	return reply.token;
}

export async function nativeChunk(
	token: string,
	index: number,
	offset: number,
	data: string
): Promise<void> {
	const target = port();
	if (!target) throw new Error('Android queue unavailable');
	await request(target, { op: 'chunk', token, index, offset, data });
}

export async function nativeCommit(token: string): Promise<string> {
	const target = port();
	if (!target) throw new Error('Android queue unavailable');
	const reply = await request(target, { op: 'commit', token });
	if (typeof reply.native_id !== 'string' || !reply.native_id)
		throw new Error('Android queue: invalid commit');
	return reply.native_id;
}

export async function nativeAbort(token: string): Promise<void> {
	const target = port();
	if (!target) return;
	try {
		await request(target, { op: 'abort', token });
	} catch {
		// Отмена не должна маскировать первичную ошибку передачи.
	}
}

export async function nativeCancel(clientId: string): Promise<void> {
	const target = port();
	if (!target) return;
	await request(target, { op: 'cancel', client_id: clientId });
}

export async function nativeStatus(): Promise<{
	records: NativeQueueStatus[];
	statuses: NativeQueueStatus[];
}> {
	const target = port();
	if (!target) return { records: [], statuses: [] };
	const reply = await request(target, { op: 'status' });
	return { records: statuses(reply.records), statuses: statuses(reply.statuses) };
}

/** Выход из учётной записи: её очередь не должна уйти после выхода. */
export async function nativeClear(origin: string): Promise<void> {
	const target = port();
	if (!target) return;
	await request(target, { op: 'clear', origin });
}
