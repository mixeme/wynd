interface PhotoLocationPort {
	postMessage(message: string): void;
	onmessage: ((event: { data: string }) => void) | null;
}
interface Reply {
	id: number;
	protocol?: number;
	token?: string;
	allowed?: boolean;
	offerChoice?: boolean;
	mode?: string;
	error?: string;
}
export type PhotoPickerMode = 'documents' | 'gallery';
interface Pending {
	resolve: (reply: Reply) => void;
	reject: (error: Error) => void;
	timer: ReturnType<typeof setTimeout>;
}
let sequence = 0;
const prefix = 'application/x-wynd-original-';
const connections = new WeakMap<PhotoLocationPort, Map<number, Pending>>();
const withoutLocation = new WeakSet<File>();
function port(): PhotoLocationPort | undefined {
	if (typeof window === 'undefined') return;
	const value = (window as unknown as { __wyndPhotoLocation?: PhotoLocationPort })
		.__wyndPhotoLocation;
	return typeof value?.postMessage === 'function' ? value : undefined;
}
export function androidPhotoLocationAvailable(): boolean {
	return !!port();
}

function request(target: PhotoLocationPort, value: Record<string, unknown>): Promise<Reply> {
	let pending = connections.get(target);
	if (!pending) {
		pending = new Map();
		connections.set(target, pending);
		const callbacks = pending;
		target.onmessage = (event) => {
			try {
				const reply: Reply = JSON.parse(event.data),
					callback = callbacks.get(reply.id);
				if (!callback) return;
				clearTimeout(callback.timer);
				callbacks.delete(reply.id);
				if (reply.error) callback.reject(new Error('Photo selection unavailable'));
				else callback.resolve(reply);
			} catch {
				/* Некорректный ответ не разрешает доступ к оригиналам. */
			}
		};
	}
	return new Promise((resolve, reject) => {
		const id = ++sequence;
		const timer = setTimeout(() => {
			pending.delete(id);
			reject(new Error('Photo selection timed out'));
		}, 5000);
		pending.set(id, { resolve, reject, timer });
		try {
			target.postMessage(JSON.stringify({ ...value, id }));
		} catch (error) {
			clearTimeout(timer);
			pending.delete(id);
			reject(error);
		}
	});
}

/** `undefined` оставляет штатный выбор оболочке, в том числе более старой версии. */
export async function photoPickerPreference(
	mode?: PhotoPickerMode
): Promise<PhotoPickerMode | 'ask' | undefined> {
	const target = port();
	if (!target) return;
	const reply = await request(target, { op: 'picker', mode }).catch((error) => {
		if (mode) throw error;
	});
	if (reply?.protocol !== 1 || reply.offerChoice !== true) return;
	return reply.mode === 'gallery' || reply.mode === 'documents' ? reply.mode : 'ask';
}

/** Одноразовая связь с input. Байты возвращает обычный File API WebView. */
export async function originalPhotoAccept(accept: string): Promise<string> {
	const target = port();
	if (!target) return accept;
	const reply = await request(target, { op: 'arm' });
	if (reply.protocol !== 1) return accept;
	if (
		typeof reply.token !== 'string' ||
		reply.token.length !== 36 ||
		!/^[0-9a-f]{8}(-[0-9a-f]{4}){3}-[0-9a-f]{12}$/.test(reply.token)
	)
		throw new Error('Invalid photo selection capability');
	return `${accept},${prefix}${reply.token}`;
}

/** SAF иногда отдаёт GPS и без ACCESS_MEDIA_LOCATION. Отказ не превращаем в место записи. */
export async function applyPhotoLocationChoice(files: File[], accept: string): Promise<void> {
	const token = accept
		.split(',')
		.map((value) => value.trim())
		.find((value) => value.startsWith(prefix))
		?.slice(prefix.length);
	if (!token) return;
	const target = port();
	if (!target) throw new Error('Photo selection unavailable');
	const reply = await request(target, { op: 'result', token });
	if (reply.protocol !== 1 || typeof reply.allowed !== 'boolean')
		throw new Error('Photo selection result unavailable');
	if (!reply.allowed) suppressPhotoLocation(files);
}
export function suppressPhotoLocation(files: File[]): void {
	for (const file of files) withoutLocation.add(file);
}
export function photoLocationAllowed(file: File): boolean {
	return !withoutLocation.has(file);
}
