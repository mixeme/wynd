import { MAX_TEXT_BYTES, textByteLength } from '$lib/journal/mentions';

interface SharePort {
	postMessage(message: string): void;
	onmessage: ((event: { data: string }) => void) | null;
}
export interface IncomingShare {
	token: string;
	text: string;
	files: { name: string; type: string; size: number; path: string }[];
}
interface Reply {
	id: number;
	protocol: number;
	pending?: IncomingShare | null;
	finished?: boolean;
}
const uuid = /^[0-9a-f]{8}(-[0-9a-f]{4}){3}-[0-9a-f]{12}$/;
let sequence = 0;
let finishing: Promise<void> | undefined;
const connections = new WeakMap<
	SharePort,
	Map<
		number,
		{ resolve: (value: Reply) => void; reject: () => void; timer: ReturnType<typeof setTimeout> }
	>
>();
function port(): SharePort | undefined {
	if (typeof window === 'undefined') return;
	const target = (window as unknown as { __wyndIncomingShare?: SharePort }).__wyndIncomingShare;
	return typeof target?.postMessage === 'function' ? target : undefined;
}
export function incomingShareAvailable(): boolean {
	return !!port();
}
export function shareToken(value: string | null): value is string {
	return !!value && uuid.test(value);
}
async function request(op: string, token?: string): Promise<Reply> {
	const target = port();
	if (!target) throw new Error('Incoming sharing unavailable');
	let pending = connections.get(target);
	if (!pending) {
		pending = new Map();
		connections.set(target, pending);
		const callbacks = pending;
		target.onmessage = ({ data }) => {
			try {
				const value: Reply = JSON.parse(data),
					callback = callbacks.get(value.id);
				if (!callback) return;
				callbacks.delete(value.id);
				clearTimeout(callback.timer);
				if (value.protocol === 1) callback.resolve(value);
				else callback.reject();
			} catch {
				/* Повреждённый ответ не даёт доступ к данным. */
			}
		};
	}
	return new Promise((resolve, reject) => {
		const id = ++sequence;
		const fail = () => reject(new Error('Incoming sharing unavailable'));
		const timer = setTimeout(() => {
			pending.delete(id);
			fail();
		}, 5000);
		pending.set(id, { resolve, reject: fail, timer });
		try {
			target.postMessage(JSON.stringify({ id, op, token }));
		} catch {
			clearTimeout(timer);
			pending.delete(id);
			fail();
		}
	});
}
export async function peekIncomingShare(): Promise<IncomingShare | null> {
	// Уход по системной «Назад» не должен опередить удаление и открыть выбор снова.
	if (finishing) await finishing.catch(() => {});
	if (!port()) return null;
	const value = (await request('peek')).pending;
	if (value == null) return null;
	if (
		!shareToken(value.token) ||
		typeof value.text !== 'string' ||
		textByteLength(value.text) > MAX_TEXT_BYTES ||
		!Array.isArray(value.files) ||
		value.files.length > 32
	)
		throw new Error('Invalid incoming share');
	let total = 0;
	for (const [index, item] of value.files.entries()) {
		if (
			typeof item.name !== 'string' ||
			!item.name ||
			!/^(image|video)\/[a-z0-9!#$&^_.+-]+$/.test(item.type) ||
			!Number.isSafeInteger(item.size) ||
			item.size < 0 ||
			item.path !== `/.wynd-android-share/${value.token}/${index}`
		)
			throw new Error('Invalid incoming media');
		total += item.size;
	}
	if (total > 512 * 1024 * 1024 || (!value.text && !value.files.length))
		throw new Error('Invalid incoming share');
	return value;
}
export function finishIncomingShare(token: string): Promise<void> {
	const previous = finishing;
	const operation = (async () => {
		if (previous) await previous.catch(() => {});
		if (!shareToken(token) || !(await request('finish', token)).finished)
			throw new Error('Incoming share changed');
	})();
	finishing = operation;
	return operation.finally(() => {
		if (finishing === operation) finishing = undefined;
	});
}
export async function loadIncomingShare(token: string): Promise<{ text: string; files: File[] }> {
	const share = await peekIncomingShare();
	if (!share || share.token !== token) throw new Error('Incoming share changed');
	const files: File[] = [];
	// По одному: fetch читает native InputStream, без base64 в сообщениях моста.
	for (const item of share.files) {
		const response = await fetch(item.path, {
			cache: 'no-store',
			credentials: 'omit',
			redirect: 'error'
		});
		if (!response.ok) throw new Error('Incoming media unavailable');
		const blob = await response.blob();
		if (blob.size !== item.size) throw new Error('Incomplete incoming media');
		files.push(new File([blob], item.name, { type: item.type }));
	}
	return { text: share.text, files };
}
export function appendSharedText(draft: string, shared: string): string {
	const text = draft && shared ? `${draft}\n\n${shared}` : draft || shared;
	if (textByteLength(text) > MAX_TEXT_BYTES) throw new Error('Combined text too long');
	return text;
}
