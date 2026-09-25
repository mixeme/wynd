import { describe, expect, it } from 'vitest';
import { ApiError } from '$lib/api/client';
import { AUTH_ERROR_HINTS, authErrorHint } from '$lib/auth/auth';

describe('authErrorHint', () => {
	it('asks for a host when SMTP is not saved', () => {
		expect(authErrorHint(new ApiError(400, 'smtp_not_configured'))).toBe(
			AUTH_ERROR_HINTS.smtp_not_configured
		);
	});

	it('explains a silent SMTP timeout', () => {
		expect(
			authErrorHint(new ApiError(502, 'smtp_failed', 'mail: send failed: mail: dial x: i/o timeout'))
		).toBe('Сервер почты не ответил. Проверьте хост и порт.');
	});

	it('explains a rejected login', () => {
		expect(
			authErrorHint(new ApiError(502, 'smtp_failed', 'mail: send failed: mail: auth: 535'))
		).toBe('Сервер не принял логин или пароль.');
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
