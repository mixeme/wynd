import { ApiError, apiJson } from '$lib/api/client';
import { listSessions } from '$lib/idb/db';

function pushAvailable(): boolean {
	return (
		typeof window !== 'undefined' &&
		window.isSecureContext &&
		'serviceWorker' in navigator &&
		'PushManager' in window
	);
}

function urlBase64ToUint8Array(base64: string): Uint8Array {
	const padding = '='.repeat((4 - (base64.length % 4)) % 4);
	const raw = atob((base64 + padding).replace(/-/g, '+').replace(/_/g, '/'));
	const output = new Uint8Array(raw.length);
	for (let i = 0; i < raw.length; i++) {
		output[i] = raw.charCodeAt(i);
	}
	return output;
}

async function postSubscription(
	origin: string,
	method: 'POST' | 'DELETE',
	sub: PushSubscription
): Promise<void> {
	const json = sub.toJSON();
	await apiJson(origin, '/push/subscribe', {
		method,
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({
			endpoint: json.endpoint,
			p256dh: json.keys?.p256dh,
			auth: json.keys?.auth
		})
	});
}

/** Register a Web Push subscription. No-op without secure context or service worker. */
export async function subscribePush(
	origin: string,
	applicationServerKey: string
): Promise<boolean> {
	if (!pushAvailable() || !applicationServerKey) return false;
	const reg = await navigator.serviceWorker.getRegistration();
	if (!reg) return false;
	const sub =
		(await reg.pushManager.getSubscription()) ??
		(await reg.pushManager.subscribe({
			userVisibleOnly: true,
			applicationServerKey: urlBase64ToUint8Array(applicationServerKey) as BufferSource
		}));
	await postSubscription(origin, 'POST', sub);
	return true;
}

export interface BrowserPushSubscription {
	endpoint: string;
	p256dh: string;
	auth: string;
}

/**
 * Подписка этого браузера для адресной проверки из панели. Разрешение
 * спрашивается первым шагом: Firefox показывает запрос только внутри жеста
 * пользователя, а после сетевых ожиданий жест уже истёк.
 */
export async function browserPushSubscription(
	fetchKey: () => Promise<string>
): Promise<BrowserPushSubscription> {
	if (!pushAvailable()) throw new ApiError(0, 'push_unavailable');
	const permission = await Notification.requestPermission();
	if (permission !== 'granted') throw new ApiError(0, 'push_permission_denied');
	const reg = await navigator.serviceWorker.ready;
	const sub =
		(await reg.pushManager.getSubscription()) ??
		(await reg.pushManager.subscribe({
			userVisibleOnly: true,
			applicationServerKey: urlBase64ToUint8Array(await fetchKey()) as BufferSource
		}));
	const json = sub.toJSON();
	return {
		endpoint: json.endpoint ?? '',
		p256dh: json.keys?.p256dh ?? '',
		auth: json.keys?.auth ?? ''
	};
}

export async function unsubscribePush(origin: string): Promise<void> {
	if (!pushAvailable()) return;
	const reg = await navigator.serviceWorker.getRegistration();
	const sub = await reg?.pushManager.getSubscription();
	if (!sub) return;
	await postSubscription(origin, 'DELETE', sub);
	await sub.unsubscribe();
}

/** Best-effort subscribe for every session. Quiet without SW / VAPID / HTTPS. */
export async function initPush(): Promise<void> {
	if (!pushAvailable()) return;
	const sessions = await listSessions();
	if (!sessions.length) return;
	for (const session of sessions) {
		try {
			const info = await apiJson<{ vapid_public_key?: string }>(session.origin, '/instance');
			if (!info.vapid_public_key) continue;
			await subscribePush(session.origin, info.vapid_public_key);
		} catch {
			/* Quiet without VAPID / permission / network. */
		}
	}
}
