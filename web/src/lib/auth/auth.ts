import { apiJson, ApiError, normalizeOrigin } from '$lib/api/client';
import { getSession, putAdminSession, putSession, type SessionRecord } from '$lib/idb/db';
import { startSync } from '$lib/sync/sync';
import { rememberSourceUrl } from '$lib/instance/source.svelte';
import type { PendingAuth, PendingAuthFlow } from './pending';

export interface InstanceInfo {
	name: string;
	version: string;
	registration_mode: 'open' | 'invite' | 'closed';
	loopback: boolean;
	bootstrapped: boolean;
	code_delivery?: 'log' | 'mail';
	source_url?: string;
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
	push_not_configured: 'Нет VAPID-ключей — перезапустите проверку',
	push_no_subscriptions: 'Этот браузер не подписан на уведомления',
	push_failed: 'Push-сервис браузера не принял уведомление',
	push_unavailable: 'Этот браузер не умеет push-уведомления: нужен HTTPS и service worker',
	push_permission_denied: 'Уведомления для этого сайта запрещены — разрешите их в настройках браузера',
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

/**
 * На устройстве один вход на сервер: входы хранятся по адресу сервера, и
 * вторая почта на том же адресе молча заменила бы первую вместе с её
 * кругами. Поэтому код для другой почты не запрашиваем — говорим, что делать.
 */
export class SecondAccountError extends Error {
	constructor(readonly email: string) {
		super('second account on the same server');
	}
}

export function secondAccountHint(email: string): string {
	return `На этом сервере вы уже вошли как ${email}. Две учётные записи на одном сервере на одном устройстве держать нельзя. Чтобы войти другой почтой, сначала выйдите: «Настройки» → «Серверы».`;
}

/** Отказ, если на этом сервере уже есть вход другой почтой. */
export async function refuseSecondAccount(origin: string, email: string): Promise<void> {
	const session = await getSession(normalizeOrigin(origin));
	if (session && session.email.trim().toLowerCase() !== email.trim().toLowerCase()) {
		throw new SecondAccountError(session.email);
	}
}

export function authErrorHint(err: unknown): string {
	if (err instanceof SecondAccountError) return secondAccountHint(err.email);
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

// detail — классифицированная причина от сервера (dial, tls,
// starttls_required, auth, protocol). Сырую строку сервера почты панель
// больше не получает (аудит 2026-09-22).
const SMTP_REASON_HINTS: Record<string, string> = {
	dial: 'Сервер почты не ответил. Проверьте хост и порт.',
	tls: 'Не удалось установить шифрование с сервером почты.',
	starttls_required:
		'Сервер почты не предлагает шифрование (STARTTLS). Коды входа открытым текстом не отправляем.',
	auth: 'Сервер не принял логин или пароль.'
};

function smtpFailedHint(detail?: string): string {
	return SMTP_REASON_HINTS[detail ?? ''] ?? AUTH_ERROR_HINTS.smtp_failed;
}

export async function fetchInstance(origin: string): Promise<InstanceInfo> {
	const info = await apiJson<InstanceInfo>(origin, '/instance');
	// Единственная воронка: любой запрос /instance пополняет адрес исходников
	// этого сервера, и экранам не нужно знать его самим (LIC-2).
	rememberSourceUrl(origin, info.source_url);
	return info;
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
		account_id: verify.account_id,
		signed_in_at: new Date().toISOString()
	};
	await putSession(session);
	startSync(session.origin);
	return session;
}

export async function sendAuthCode(pending: PendingAuth): Promise<PendingAuth> {
	const { origin, email, flow, inviteToken, inviteName } = pending;
	await refuseSecondAccount(origin, email);
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

export async function probeBootstrapSmtp(input: {
	token: string;
	host: string;
	port?: number;
	username?: string;
	smtp_password?: string;
	from?: string;
}): Promise<void> {
	await apiJson('', '/admin/bootstrap/smtp-test', {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(input)
	});
}

export async function bootstrapAdmin(input: {
	token: string;
	instance_name: string;
	password: string;
	public_url?: string;
	host?: string;
	port?: number;
	username?: string;
	smtp_password?: string;
	from?: string;
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
	public_url?: string;
	host?: string;
	port?: number;
	username?: string;
	smtp_password?: string;
	from?: string;
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
