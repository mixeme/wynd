import { apiJson, ApiError, normalizeOrigin } from '$lib/api/client';
import { putAdminSession, putSession, type SessionRecord } from '$lib/idb/db';
import { startSync } from '$lib/sync/sync';
import type { PendingAuth, PendingAuthFlow } from './pending';

export interface InstanceInfo {
	name: string;
	version: string;
	registration_mode: 'open' | 'invite' | 'closed';
	loopback: boolean;
	bootstrapped: boolean;
	code_delivery?: 'log' | 'mail';
}

export interface VerifyResult {
	token: string;
	account_id: string;
	email: string;
	expires_at: string;
	pending_circle_id?: string;
}

export const AUTH_ERROR_HINTS: Record<string, string> = {
	invalid: 'Проверьте адрес и попробуйте снова',
	not_found: 'Аккаунт с такой почтой не найден',
	forbidden: 'Нет доступа',
	registration_closed: 'Сервер не принимает новых участников',
	closed: 'Сервер не принимает новых участников',
	rate_limited: 'Слишком много запросов — подождите',
	too_many_attempts: 'Исчерпаны попытки ввода кода',
	expired: 'Код истёк — запросите новый',
	weak_password: 'Пароль должен быть не короче 8 символов',
	too_long: 'Слишком длинный текст',
	payload_too_large: 'Слишком большой запрос',
	smtp_not_configured: 'Сначала сохраните хост и адрес отправителя',
	smtp_failed: 'Письмо не ушло. Проверьте хост, порт, логин и пароль.',
	internal: 'Не удалось выполнить запрос',
	unknown: 'Не удалось выполнить запрос'
};

export function rateLimitedHint(retryAfterSec: number): string {
	if (retryAfterSec < 60) {
		return `Слишком много запросов. Следующий код можно запросить через ${retryAfterSec} сек.`;
	}
	const min = Math.ceil(retryAfterSec / 60);
	return `Слишком много запросов. Следующий код можно запросить через ${min} мин.`;
}

export function authErrorHint(err: unknown): string {
	if (err instanceof ApiError) {
		if (err.code === 'rate_limited' && err.retryAfterSec != null) {
			return rateLimitedHint(err.retryAfterSec);
		}
		if (err.code === 'smtp_failed') {
			return smtpFailedHint(err.detail);
		}
		return AUTH_ERROR_HINTS[err.code] ?? err.code;
	}
	return AUTH_ERROR_HINTS.unknown;
}

function smtpFailedHint(detail?: string): string {
	const d = (detail ?? '').toLowerCase();
	if (d.includes('timeout') || d.includes('deadline') || d.includes('i/o')) {
		return 'Сервер почты не ответил. Проверьте хост и порт.';
	}
	if (d.includes('auth') || d.includes('535') || d.includes('534')) {
		return 'Сервер не принял логин или пароль.';
	}
	if (d.includes('tls') || d.includes('certificate')) {
		return 'Не удалось установить шифрование с сервером почты.';
	}
	return AUTH_ERROR_HINTS.smtp_failed;
}

export async function fetchInstance(origin: string): Promise<InstanceInfo> {
	return apiJson<InstanceInfo>(origin, '/instance');
}

async function postAccepted(origin: string, path: string, body: unknown): Promise<void> {
	await apiJson(origin, path, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(body)
	});
}

export async function requestRegister(origin: string, email: string): Promise<void> {
	await postAccepted(origin, '/auth/register', { email });
}

export async function requestLoginCode(origin: string, email: string): Promise<void> {
	await postAccepted(origin, '/auth/code', { email });
}

export async function acceptInvite(
	origin: string,
	token: string,
	email: string,
	name: string
): Promise<void> {
	await postAccepted(origin, `/invites/${token}/accept`, { email, name });
}

export async function verifyCode(
	origin: string,
	email: string,
	code: string
): Promise<VerifyResult> {
	return apiJson<VerifyResult>(origin, '/auth/verify', {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ email, code })
	});
}

export async function persistSession(
	origin: string,
	verify: VerifyResult,
	instanceName: string
): Promise<SessionRecord> {
	const session: SessionRecord = {
		origin: normalizeOrigin(origin),
		name: instanceName,
		email: verify.email,
		token: verify.token,
		account_id: verify.account_id
	};
	await putSession(session);
	startSync(session.origin);
	return session;
}

export async function sendAuthCode(pending: PendingAuth): Promise<PendingAuth> {
	const { origin, email, flow, inviteToken, inviteName } = pending;
	if (flow === 'register') {
		await requestRegister(origin, email);
	} else if (flow === 'login') {
		await requestLoginCode(origin, email);
	} else if (flow === 'invite' && inviteToken) {
		await acceptInvite(origin, inviteToken, email, inviteName ?? '');
	} else {
		throw new Error('invalid pending auth flow');
	}
	return { ...pending, codeSentAt: Date.now(), retryUntil: undefined };
}

export function flowForJoin(
	mode: InstanceInfo['registration_mode'],
	hasToken: boolean
): PendingAuthFlow | 'closed' {
	if (mode === 'closed') return 'closed';
	if (hasToken) return 'invite';
	if (mode === 'open') return 'register';
	return 'closed';
}

export interface AdminLoginResult {
	token: string;
	expires_at: string;
}

export async function bootstrapAdmin(input: {
	token: string;
	instance_name: string;
	password: string;
}): Promise<void> {
	await apiJson('', '/admin/bootstrap', {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(input)
	});
}

export async function loginAdmin(password: string): Promise<AdminLoginResult> {
	return apiJson<AdminLoginResult>('', '/admin/login', {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ password })
	});
}

export async function completeBootstrap(input: {
	token: string;
	instance_name: string;
	password: string;
}): Promise<void> {
	await bootstrapAdmin(input);
	const session = await loginAdmin(input.password);
	await putAdminSession({ origin: '', token: session.token });
}

/** Revoke the participant session on the server. Errors are swallowed: the
 * local record is removed regardless, and an already-dead token is fine. */
export async function logoutSession(origin: string): Promise<void> {
	try {
		await apiJson(origin, '/auth/logout', { method: 'POST' });
	} catch {
		/* token already invalid or server unreachable */
	}
}
