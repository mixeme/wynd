import { normalizeOrigin } from '$lib/api/client';

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
	if (typeof window !== 'undefined' && parsed === window.location.origin) return '';
	if (typeof window !== 'undefined') {
		const host = window.location.hostname;
		if (
			(host === '127.0.0.1' || host === 'localhost') &&
			(parsed.includes('127.0.0.1') || parsed.includes('localhost'))
		) {
			return '';
		}
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
