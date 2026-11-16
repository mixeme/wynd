export interface AndroidUpdateStatus {
	protocol: number;
	capabilities: { updates: boolean; developmentUpdateBypass: boolean };
	version: string;
	debug: boolean;
	development: boolean;
	immediately: boolean;
	state: string;
	available: boolean;
	nextVersion: string;
	ready: boolean;
	error: string;
	blocked: boolean;
	installing: boolean;
	permission: boolean;
}

interface NativePort {
	postMessage(message: string): void;
	onmessage: ((event: { data: string }) => void) | null;
}

let connected: NativePort | undefined;
let sequence = 0;
const pending = new Map<
	number,
	{
		resolve: (status: AndroidUpdateStatus) => void;
		reject: (error: Error) => void;
		timer: ReturnType<typeof setTimeout>;
	}
>();

export function androidPort(): NativePort | undefined {
	if (typeof window === 'undefined') return undefined;
	const port = (window as unknown as { __wyndAndroid?: NativePort }).__wyndAndroid;
	return port && typeof port.postMessage === 'function' ? port : undefined;
}

export function androidRequest(method: string, value?: boolean): Promise<AndroidUpdateStatus> {
	const port = androidPort();
	if (!port) return Promise.reject(new Error('Android capability unavailable'));
	if (connected !== port) {
		connected = port;
		port.onmessage = (event) => {
			try {
				const reply = JSON.parse(event.data);
				const request = pending.get(reply.id);
				if (!request || !reply.result || typeof reply.result.version !== 'string') return;
				clearTimeout(request.timer);
				pending.delete(reply.id);
				request.resolve(reply.result);
			} catch {
				// Повреждённый ответ не завершает запрос и не заменяет состояние.
			}
		};
	}
	return new Promise((resolve, reject) => {
		const id = ++sequence;
		const timer = setTimeout(() => {
			pending.delete(id);
			reject(new Error('Android request timed out'));
		}, 180_000);
		pending.set(id, { resolve, reject, timer });
		try {
			port.postMessage(JSON.stringify({ id, method, ...(value === undefined ? {} : { value }) }));
		} catch (error) {
			clearTimeout(timer);
			pending.delete(id);
			reject(error instanceof Error ? error : new Error('Android request failed'));
		}
	});
}
