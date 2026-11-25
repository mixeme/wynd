import type { SessionRecord } from '$lib/idb/db';

/** Мост общего хранилища входов оболочки Android (C127). */
type Port = {
	postMessage(message: string): void;
	onmessage: ((event: { data: string }) => void) | null;
};

type Request = (op: string, args: object) => Promise<unknown>;
const connections = new WeakMap<Port, Request>();
let nextId = 0;

function port(): Port | undefined {
	if (typeof window === 'undefined') return;
	return (window as typeof window & { __wyndSecrets?: Port }).__wyndSecrets;
}

export function androidSecretsAvailable(): boolean {
	return typeof port()?.postMessage === 'function';
}

function connection(target: Port): Request {
	const pending = new Map<
		number,
		{
			resolve: (value: unknown) => void;
			reject: (error: Error) => void;
			timer: ReturnType<typeof setTimeout>;
		}
	>();
	target.onmessage = (event) => {
		try {
			const response = JSON.parse(event.data);
			const request = pending.get(response.id);
			if (!request) return;
			clearTimeout(request.timer);
			pending.delete(response.id);
			if (response.result) request.resolve(response.result);
			else request.reject(new Error('Android secrets unavailable'));
		} catch {
			/* Некорректный ответ не завершает запрос. */
		}
	};
	return (op, args) =>
		new Promise((resolve, reject) => {
			const id = ++nextId;
			const timer = setTimeout(() => {
				pending.delete(id);
				reject(new Error('Android secrets request timed out'));
			}, 5000);
			pending.set(id, { resolve, reject, timer });
			try {
				target.postMessage(JSON.stringify({ id, op, ...args }));
			} catch (error) {
				pending.delete(id);
				clearTimeout(timer);
				reject(error);
			}
		});
}

function request<T>(op: string, args = {}): Promise<T> {
	const target = port();
	if (!target) return Promise.reject(new Error('Android secrets unavailable'));
	let send = connections.get(target);
	if (!send) {
		send = connection(target);
		connections.set(target, send);
	}
	return send(op, args) as Promise<T>;
}

/** Все входы телефона: записи с абсолютным адресом своего сервера. */
export async function androidSecretsList(): Promise<SessionRecord[]> {
	const result = await request<{ sessions?: SessionRecord[] }>('list');
	return result.sessions ?? [];
}

export function androidSecretsPut(session: SessionRecord): Promise<void> {
	return request<object>('put', { session }).then(() => {});
}

export function androidSecretsDelete(origin: string): Promise<void> {
	return request<object>('delete', { origin }).then(() => {});
}
