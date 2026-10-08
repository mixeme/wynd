import { apiJson, ApiError, normalizeOrigin } from '$lib/api/client';
import { getSession, putSession } from '$lib/idb/db';
import { authErrorHint } from './auth';
import type { CodeDelivery } from './pending';

/**
 * Смена почты на одном сервере (кадры 7.12–7.14). Между «Серверами», формой
 * и экраном кода это живёт во вкладке: почта меняется только после кода.
 */
export interface PendingEmailChange {
	origin: string;
	serverName: string;
	host: string;
	currentEmail: string;
	/** Новая почта; пусто, пока её не назвали. */
	email: string;
	codeDelivery?: CodeDelivery;
	codeSentAt?: number;
	/** Earliest time (ms) when resend is allowed after rate_limited. */
	retryUntil?: number;
}

const KEY = 'wynd:pending-email-change';

export function loadPendingEmailChange(): PendingEmailChange | undefined {
	if (typeof sessionStorage === 'undefined') return undefined;
	try {
		const raw = sessionStorage.getItem(KEY);
		if (!raw) return undefined;
		return JSON.parse(raw) as PendingEmailChange;
	} catch {
		return undefined;
	}
}

export function savePendingEmailChange(pending: PendingEmailChange): void {
	sessionStorage.setItem(KEY, JSON.stringify(pending));
}

export function clearPendingEmailChange(): void {
	sessionStorage.removeItem(KEY);
}

export function sameEmail(a: string, b: string): boolean {
	return a.trim().toLowerCase() === b.trim().toLowerCase();
}

export async function requestEmailChange(origin: string, email: string): Promise<void> {
	await apiJson(origin, '/auth/email', {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ email })
	});
}

/** Верный код: сервер сменил почту — запись входа на устройстве идёт следом. */
export async function confirmEmailChange(origin: string, code: string): Promise<string> {
	const res = await apiJson<{ email: string }>(origin, '/auth/email/verify', {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ code })
	});
	const session = await getSession(normalizeOrigin(origin));
	if (session) await putSession({ ...session, email: res.email });
	return res.email;
}

const EMAIL_CHANGE_HINTS: Record<string, string> = {
	conflict: 'Эта почта уже есть на этом сервере',
	not_found: 'Код уже не действует — запросите новый',
	forbidden: 'Вход на этот сервер закрыт — войдите заново'
};

/** `step` — где случился отказ: «invalid» у адреса и у кода значит разное. */
export function emailChangeErrorHint(err: unknown, step: 'email' | 'code'): string {
	if (err instanceof ApiError) {
		if (err.code === 'invalid' && step === 'code') return 'Код не подошёл';
		const hint = EMAIL_CHANGE_HINTS[err.code];
		if (hint) return hint;
	}
	return authErrorHint(err);
}
