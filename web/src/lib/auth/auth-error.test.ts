import { describe, expect, it } from 'vitest';
import { ApiError } from '$lib/api/client';
import { AUTH_ERROR_HINTS, authErrorHint } from '$lib/auth/auth';

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
