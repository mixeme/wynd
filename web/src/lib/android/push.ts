import type { SessionRecord } from '$lib/idb/db';

type Port = {
	postMessage(message: string): void;
	onmessage: ((event: { data: string }) => void) | null;
};

export interface AndroidPushStatus {
	enabled: boolean;
	allowed: boolean;
	reason: '' | 'permission' | 'distributor';
}

export interface AndroidPushSubscription {
	endpoint: string;
	p256dh: string;
	auth: string;
}

interface Registration {
	subscription?: AndroidPushSubscription;
	published?: string;
}

type Request = (op: string, args: object) => Promise<unknown>;
const connections = new WeakMap<Port, Request>();
let nextId = 0;

function port(): Port | undefined {
	if (typeof window === 'undefined') return;
	return (window as typeof window & { __wyndPush?: Port }).__wyndPush;
}

export function androidPushAvailable(): boolean {
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
			else request.reject(new Error('Android notifications unavailable'));
		} catch {
			/* Invalid responses cannot complete a request. */
		}
	};
	return (op, args) =>
		new Promise((resolve, reject) => {
			const id = ++nextId;
			const timer = setTimeout(
				() => {
					pending.delete(id);
					reject(new Error('Android notifications request timed out'));
				},
				op === 'enable' ? 180_000 : 5000
			);
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
	if (!target) return Promise.reject(new Error('Android notifications unavailable'));
	let send = connections.get(target);
	if (!send) {
		send = connection(target);
		connections.set(target, send);
	}
	return send(op, args) as Promise<T>;
}

export function androidPushStatus(op: 'status' | 'enable' | 'disable' = 'status') {
	return request<AndroidPushStatus>(op);
}

function identity(session: SessionRecord) {
	return { account: session.account_id, origin: session.origin || window.location.origin };
}

export function ensureAndroidPush(session: SessionRecord, vapid: string) {
	return request<Registration>('ensure', { ...identity(session), vapid });
}

export function markAndroidPushPublished(session: SessionRecord, endpoint: string) {
	return request<Registration>('published', { ...identity(session), endpoint });
}

export function removeAndroidPush(session: SessionRecord) {
	return request<Registration>('remove', identity(session));
}
