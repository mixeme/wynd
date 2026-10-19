import { describe, expect, it } from 'vitest';
import { shouldOpenPaywall } from './client';

describe('shouldOpenPaywall', () => {
	it('opens the paywall from a participant screen', () => {
		expect(shouldOpenPaywall('/circles/abc/posts', '/circles/abc')).toBe(true);
	});

	it('never leaves the admin panel', () => {
		// Фоновая проверка сессии участника идёт и на /admin (владелец сервера без подписки).
		expect(shouldOpenPaywall('/circles', '/admin/pay/subscription')).toBe(false);
		expect(shouldOpenPaywall('/admin/access', '/circles')).toBe(false);
	});

	it('does not reload /circles over and over', () => {
		expect(shouldOpenPaywall('/circles', '/circles')).toBe(false);
	});
});
