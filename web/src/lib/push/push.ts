import { ApiError, apiJson, apiPath } from '$lib/api/client';
import { getSession, listSessions, type SessionRecord } from '$lib/idb/db';
import {
	androidPushAvailable,
	androidPushStatus,
	ensureAndroidPush,
	markAndroidPushPublished,
	removeAndroidPush,
	type AndroidPushSubscription
} from '$lib/android/push';

let androidQueue = Promise.resolve();
let androidListening = false;
const signingOut = new Set<string>();

function androidUpdate<T>(fn: () => Promise<T>): Promise<T> {
	const result = androidQueue.then(fn);
	androidQueue = result.then(
		() => {},
		() => {}
	);
	return result;
}

function sessionID(session: SessionRecord): string {
	return JSON.stringify([session.origin, session.account_id, session.signed_in_at]);
}

// Background registration is account-bound. An in-flight request after logout must
// not take a fresh token from IndexedDB and move an endpoint to a different account.
async function androidSubscriptionRequest(
	session: SessionRecord,
	method: 'POST' | 'DELETE',
	body: AndroidPushSubscription | { endpoint: string }
): Promise<void> {
	const controller = new AbortController();
	const timer = setTimeout(() => controller.abort(), 10000);
	try {
		const response = await fetch(apiPath(session.origin, '/push/subscribe'), {
			method,
			headers: {
				Authorization: 'Bearer ' + session.token,
				'Content-Type': 'application/json'
			},
			body: JSON.stringify(body),
			signal: controller.signal
		});
		if (!response.ok) throw new Error('Android notification subscription request failed');
	} finally {
		clearTimeout(timer);
	}
}

async function removeAndroidRegistration(session: SessionRecord): Promise<() => Promise<void>> {
	const registration = await removeAndroidPush(session);
	const endpoints = new Set([registration.subscription?.endpoint, registration.published]);
	endpoints.delete(undefined);
	return async () => {
		await Promise.allSettled(
			[...endpoints].map((endpoint) =>
				endpoint ? androidSubscriptionRequest(session, 'DELETE', { endpoint }) : Promise.resolve()
			)
		);
	};
}

async function syncAndroidPush(): Promise<void> {
	const status = await androidPushStatus();
	if (!status.enabled || !status.allowed) return;
	await Promise.allSettled(
		(await listSessions())
			.filter((session) => !signingOut.has(sessionID(session)))
			.map(async (session) => {
				const controller = new AbortController();
				const timer = setTimeout(() => controller.abort(), 10000);
				const { vapid_public_key: vapid } = await apiJson<{ vapid_public_key?: string }>(
					session.origin,
					'/instance',
					{ signal: controller.signal }
				).finally(() => clearTimeout(timer));
				if (!vapid || signingOut.has(sessionID(session))) return;
				const { published, subscription } = await ensureAndroidPush(session, vapid);
				if (!subscription || signingOut.has(sessionID(session))) return;
				await androidSubscriptionRequest(session, 'POST', subscription);
				if (published && published !== subscription.endpoint) {
					await androidSubscriptionRequest(session, 'DELETE', { endpoint: published });
				}
				await markAndroidPushPublished(session, subscription.endpoint);
			})
	);
}

export function setAndroidPushEnabled(enabled: boolean) {
	return androidUpdate(async () => {
		if (enabled) {
			const status = await androidPushStatus('enable');
			if (status.enabled && status.allowed) await syncAndroidPush();
			return status;
		}
		const removals = await Promise.allSettled(
			(await listSessions()).map(removeAndroidRegistration)
		);
		const status = await androidPushStatus('disable');
		for (const removal of removals) {
			if (removal.status === 'fulfilled') void removal.value();
		}
		return status;
	});
}

export function refreshAndroidPush(): void {
	if (androidPushAvailable()) void androidUpdate(syncAndroidPush).catch(() => {});
}

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
	if (androidPushAvailable()) {
		const session = await getSession(origin);
		if (session) {
			signingOut.add(sessionID(session));
			const removeOnServer = await androidUpdate(() => removeAndroidRegistration(session));
			await removeOnServer();
		}
		return;
	}
	if (!pushAvailable()) return;
	const reg = await navigator.serviceWorker.getRegistration();
	const sub = await reg?.pushManager.getSubscription();
	if (!sub) return;
	await postSubscription(origin, 'DELETE', sub);
	await sub.unsubscribe();
}

/** Best-effort subscribe for every session. Quiet without SW / VAPID / HTTPS. */
export async function initPush(): Promise<void> {
	if (androidPushAvailable()) {
		if (!androidListening) {
			androidListening = true;
			window.addEventListener('wynd:android-push', refreshAndroidPush);
			window.addEventListener('online', refreshAndroidPush);
			document.addEventListener('visibilitychange', () => {
				if (document.visibilityState === 'visible') refreshAndroidPush();
			});
		}
		refreshAndroidPush();
		return;
	}
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
