/**
 * Нативная очередь (план 50, этап 5.1): записи без вложений уходят через
 * WorkManager и после закрытия окна. Страница отдаёт запись и токен только
 * главному фрейму выбранного origin; native хранит их в private-файле.
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
	state: 'pending' | 'sent' | 'failed' | 'unauthorized';
	error?: string;
}

interface NativeQueuePort {
	postMessage(message: string): void;
	onmessage: ((event: { data: string }) => void) | null;
}

interface Reply {
	id: number;
	native_id?: string;
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
	return {
		client_id: item.client_id,
		state: item.state,
		error: typeof item.error === 'string' ? item.error : undefined
	};
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
