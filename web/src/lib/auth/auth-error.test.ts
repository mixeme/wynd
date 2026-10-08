import { describe, expect, it } from 'vitest';
import { ApiError } from '$lib/api/client';
import {
	AUTH_ERROR_HINTS,
	SecondAccountError,
	authErrorHint,
	refuseSecondAccount
} from '$lib/auth/auth';
import { deleteSession, putSession } from '$lib/idb/db';

describe('authErrorHint', () => {
	it('asks for a host when SMTP is not saved', () => {
		expect(authErrorHint(new ApiError(400, 'smtp_not_configured'))).toBe(
			AUTH_ERROR_HINTS.smtp_not_configured
		);
	});

	// detail — метка причины от сервера, а не его сырой ответ.
	it('explains a silent SMTP timeout', () => {
		expect(authErrorHint(new ApiError(502, 'smtp_failed', 'dial'))).toBe(
			'Сервер почты не ответил. Проверьте хост и порт.'
		);
	});

	it('explains a rejected login', () => {
		expect(authErrorHint(new ApiError(502, 'smtp_failed', 'auth'))).toBe(
			'Сервер не принял логин или пароль.'
		);
	});

	it('explains a relay without STARTTLS', () => {
		expect(authErrorHint(new ApiError(502, 'smtp_failed', 'starttls_required'))).toContain(
			'STARTTLS'
		);
	});

	it('falls back on an unknown reason', () => {
		expect(authErrorHint(new ApiError(502, 'smtp_failed', 'protocol'))).toBe(
			AUTH_ERROR_HINTS.smtp_failed
		);
	});

	it('names the wait after a rate limit', () => {
		expect(authErrorHint(new ApiError(429, 'rate_limited', undefined, 45))).toBe(
			'Слишком много запросов. Следующий код можно запросить через 45 сек.'
		);
		expect(authErrorHint(new ApiError(429, 'rate_limited', undefined, 120))).toBe(
			'Слишком много запросов. Следующий код можно запросить через 2 мин.'
		);
	});
});

// Входы хранятся по адресу сервера: вторая почта заменила бы первую.
describe('вторая учётная запись на том же сервере', () => {
	const origin = 'https://home.example.org';
	const session = {
		origin,
		name: 'Дом Ани',
		email: 'anya@example.com',
		token: 't',
		account_id: 'a',
		signed_in_at: '2026-10-08T00:00:00Z'
	};

	it('другой почте — отказ с прежней почтой и путём к выходу', async () => {
		await putSession(session);
		const err = await refuseSecondAccount(origin + '/', 'petya@example.com').catch((e) => e);
		expect(err).toBeInstanceOf(SecondAccountError);
		expect(authErrorHint(err)).toContain('anya@example.com');
		expect(authErrorHint(err)).toContain('«Серверы»');
		await deleteSession(origin);
	});

	it('та же почта входит заново, регистр не важен', async () => {
		await putSession(session);
		await expect(refuseSecondAccount(origin, 'Anya@Example.com')).resolves.toBeUndefined();
		await deleteSession(origin);
	});

	it('на сервере без входа отказа нет', async () => {
		await expect(refuseSecondAccount(origin, 'petya@example.com')).resolves.toBeUndefined();
	});
});
