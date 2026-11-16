/** Выбор источника интерфейса. Этот модуль тоже приходит с текущего сервера. */
interface InterfacePort {
	postMessage(message: string): void;
	onmessage: ((event: { data: string }) => void) | null;
}
export interface CheckedInterface {
	origin: string;
	name: string;
	allow_name_erasure: boolean;
	ticket: string;
}
let sequence = 0;
let scannedInvite = '';
const connections = new WeakMap<
	InterfacePort,
	Map<
		number,
		{
			resolve: (value: Record<string, unknown>) => void;
			reject: (error: Error) => void;
			timer: ReturnType<typeof setTimeout>;
		}
	>
>();

function port(): InterfacePort | undefined {
	if (typeof window === 'undefined') return;
	const value = (window as unknown as { __wyndInterface?: InterfacePort }).__wyndInterface;
	return typeof value?.postMessage === 'function' ? value : undefined;
}
export function interfaceAvailable(): boolean {
	return !!port();
}
export function saveScannedInvite(value: string): void {
	scannedInvite = value;
}
export function takeScannedInvite(): string {
	const value = scannedInvite;
	scannedInvite = '';
	return value;
}

async function request(method: string, input = {}): Promise<Record<string, unknown>> {
	const target = port();
	if (!target) throw new Error('unsupported');
	let callbacks = connections.get(target);
	if (!callbacks) {
		callbacks = new Map();
		connections.set(target, callbacks);
		const pending = callbacks;
		target.onmessage = ({ data }) => {
			try {
				const reply = JSON.parse(data),
					callback = pending.get(reply.id);
				if (!callback || reply.protocol !== 1) return;
				pending.delete(reply.id);
				clearTimeout(callback.timer);
				if (reply.error || !reply.result || typeof reply.result !== 'object')
					callback.reject(new Error(reply.error || 'invalid'));
				else callback.resolve(reply.result);
			} catch {
				/* Повреждённый ответ не меняет выбор. */
			}
		};
	}
	return new Promise((resolve, reject) => {
		const id = ++sequence;
		const timer = setTimeout(() => {
			callbacks.delete(id);
			reject(new Error('timeout'));
		}, 25000);
		callbacks.set(id, { resolve, reject, timer });
		try {
			target.postMessage(JSON.stringify({ id, method, ...input }));
		} catch {
			clearTimeout(timer);
			callbacks.delete(id);
			reject(new Error('unavailable'));
		}
	});
}
export async function checkInterface(address: string): Promise<CheckedInterface> {
	const value = await request('check', { address });
	if (
		typeof value.origin !== 'string' ||
		typeof value.name !== 'string' ||
		typeof value.ticket !== 'string' ||
		typeof value.allow_name_erasure !== 'boolean'
	)
		throw new Error('invalid');
	return value as unknown as CheckedInterface;
}
export async function selectInterface(ticket: string, path = '/'): Promise<void> {
	if ((await request('select', { ticket, path })).selected !== true) throw new Error('invalid');
}
export function cancelInterfaceCheck(): void {
	if (port()) void request('cancel').catch(() => {});
}

/** Для приглашения нужен абсолютный адрес. Проверка карточки не получает его токен. */
export function interfaceInvite(text: string): { origin: string; path: string } | null {
	try {
		const raw = text.trim();
		if (raw.startsWith('/') || raw.includes('\\') || [...raw].some((c) => c.charCodeAt(0) <= 32))
			return null;
		const url = new URL(raw.includes('://') ? raw : `https://${raw}`);
		if (
			!['https:', 'http:'].includes(url.protocol) ||
			url.username ||
			url.password ||
			url.hash ||
			!/^\/(invite|join)\/[A-Za-z0-9_-]+$/.test(url.pathname) ||
			!/^[?A-Za-z0-9_~%=&.+-]*$/.test(url.search)
		)
			return null;
		return { origin: url.origin, path: url.pathname + url.search };
	} catch {
		return null;
	}
}
