import { normalizeOrigin } from '$lib/api/client';

/** Same rule as config.IsLoopback: localhost and loopback IPs, with or without a scheme. */
export function isLoopbackPublicURL(raw: string): boolean {
	let s = raw.trim();
	if (!s) return false;
	if (!/^[a-zA-Z][a-zA-Z0-9+.-]*:\/\//.test(s)) s = `https://${s}`;
	try {
		const host = new URL(s).hostname.replace(/^\[|\]$/g, '').toLowerCase();
		if (host === 'localhost' || host === '::1') return true;
		return /^127(?:\.\d{1,3}){3}$/.test(host);
	} catch {
		return false;
	}
}

export function parseServerInput(raw: string): string {
	let s = raw.trim();
	if (!s) return '';
	if (!/^https?:\/\//i.test(s)) s = `https://${s}`;
	try {
		return new URL(s).origin;
	} catch {
		return '';
	}
}

/** Map user-entered server address to API origin ('' = same host / dev proxy). */
export function resolveServerOrigin(input: string): string {
	const parsed = parseServerInput(input);
	if (!parsed) return '';
	if (typeof window !== 'undefined') {
		if (parsed === window.location.origin) return '';
		// Поле входа показывает свой сервер без схемы (displayHost). Дописанный
		// https не совпадал со страницей по http (LAN, loopback) — вход уходил
		// на https и падал «Сервер не отвечает».
		const typedScheme = /^https?:\/\//i.test(input.trim());
		if (!typedScheme && new URL(parsed).host === window.location.host) return '';
	}
	return normalizeOrigin(parsed);
}

export function displayHost(origin: string): string {
	if (!origin) {
		if (typeof window !== 'undefined') return window.location.host;
		return '';
	}
	try {
		return new URL(origin).host;
	} catch {
		return origin;
	}
}
