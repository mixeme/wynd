/**
 * Выбранный `.wynd` в Android-оболочке (план 50, 5.4): native помнит
 * постоянное право чтения URI и отдаёт файл странице через тот же origin.
 * В браузере моста нет — выбор файла остаётся прежним.
 */
interface ArchivePort {
	postMessage(message: string): void;
	onmessage: ((event: { data: string }) => void) | null;
}

interface ArchiveInfo {
	name?: string;
	size?: number;
	url?: string;
}

interface Reply {
	id: number;
	archive?: unknown;
	ok?: boolean;
	error?: string;
}

const TIMEOUT_MS = 10_000;
let sequence = 0;
const connections = new WeakMap<
	ArchivePort,
	Map<
		number,
		{
			resolve: (reply: Reply) => void;
			reject: (error: Error) => void;
			timer: ReturnType<typeof setTimeout>;
		}
	>
>();

function port(): ArchivePort | undefined {
	if (typeof window === 'undefined') return undefined;
	const candidate = (window as unknown as { __wyndArchive?: ArchivePort }).__wyndArchive;
	return typeof candidate?.postMessage === 'function' ? candidate : undefined;
}

export function androidArchiveAvailable(): boolean {
	return !!port();
}

function request(target: ArchivePort, value: Record<string, unknown>): Promise<Reply> {
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
					callback.reject(new Error(`Android archive: ${reply.error}`));
					return;
				}
				callback.resolve(reply);
			} catch {
				/* Повреждённый ответ не завершает запрос. */
			}
		};
	}
	return new Promise((resolve, reject) => {
		const id = ++sequence;
		const timer = setTimeout(() => {
			pending.delete(id);
			reject(new Error('Android archive timed out'));
		}, TIMEOUT_MS);
		pending.set(id, { resolve, reject, timer });
		try {
			target.postMessage(JSON.stringify({ ...value, id }));
		} catch (error) {
			clearTimeout(timer);
			pending.delete(id);
			reject(error instanceof Error ? error : new Error('Android archive failed'));
		}
	});
}

function archive(value: unknown): ArchiveInfo | undefined {
	if (!value || typeof value !== 'object') return undefined;
	const item = value as ArchiveInfo;
	if (typeof item.name !== 'string' || item.name.length > 512) return undefined;
	return {
		name: item.name,
		size: typeof item.size === 'number' ? item.size : undefined,
		url: typeof item.url === 'string' ? item.url : undefined
	};
}

/** Помнит ли native выбранный .wynd (без открытия файла). */
export async function androidArchiveStatus(): Promise<ArchiveInfo | undefined> {
	const target = port();
	if (!target) return undefined;
	const reply = await request(target, { op: 'status' });
	return archive(reply.archive);
}

/** Готовит файл к чтению: native отдаёт его по пути в том же origin. */
export async function androidArchiveFile(): Promise<
	{ file: File; name: string; size: number } | undefined
> {
	const target = port();
	if (!target) return undefined;
	const reply = await request(target, { op: 'open' });
	const opened = archive(reply.archive);
	if (!opened?.url) return undefined;
	const response = await fetch(opened.url, { cache: 'no-store' });
	if (!response.ok) throw new Error(`Android archive: ${response.status}`);
	const blob = await response.blob();
	const name = opened.name ?? 'archive.wynd';
	const file = new File([blob], name, { type: blob.type || 'application/octet-stream' });
	return { file, name, size: opened.size ?? file.size };
}

/** Успешное оживление или отказ: постоянное право и токен больше не нужны. */
export async function androidArchiveForget(): Promise<void> {
	const target = port();
	if (!target) return;
	try {
		await request(target, { op: 'forget' });
	} catch {
		// Файл уже мог исчезнуть.
	}
}
