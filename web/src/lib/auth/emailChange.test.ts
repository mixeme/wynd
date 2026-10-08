import { describe, expect, it } from 'vitest';
import { ApiError } from '$lib/api/client';
import { AUTH_ERROR_HINTS } from '$lib/auth/auth';
import { emailChangeErrorHint, sameEmail } from '$lib/auth/emailChange';

describe('смена почты: отказы словами', () => {
	it('занятый адрес — два входа не сливаются', () => {
		expect(emailChangeErrorHint(new ApiError(409, 'conflict'), 'email')).toBe(
			'Эта почта уже есть на этом сервере'
		);
	});

	// «invalid» у адреса — про адрес, у кода — про код.
	it('различает неверный адрес и неверный код', () => {
		expect(emailChangeErrorHint(new ApiError(400, 'invalid'), 'email')).toBe(
			AUTH_ERROR_HINTS.invalid
		);
		expect(emailChangeErrorHint(new ApiError(400, 'invalid'), 'code')).toBe('Код не подошёл');
	});

	it('оставляет общие отказы входа как есть', () => {
		expect(emailChangeErrorHint(new ApiError(410, 'expired'), 'code')).toBe(
			AUTH_ERROR_HINTS.expired
		);
		expect(emailChangeErrorHint(new ApiError(429, 'too_many_attempts'), 'code')).toBe(
			AUTH_ERROR_HINTS.too_many_attempts
		);
	});

	it('та же почта в другом регистре — та же', () => {
		expect(sameEmail(' Anya@Example.com ', 'anya@example.com')).toBe(true);
		expect(sameEmail('anya@example.com', 'anya@newmail.example')).toBe(false);
	});
});
