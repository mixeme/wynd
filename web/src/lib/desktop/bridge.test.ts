import { afterEach, describe, expect, it } from 'vitest';
import { apiPath, normalizeOrigin } from '$lib/api/client';
import { resolveServerOrigin } from '$lib/auth/origin';
import { desktopBridge, desktopProxyBase, pageOrigin } from './bridge';

function bridge(origin: string) {
	(window as unknown as { __wyndDesktop?: { origin: string } }).__wyndDesktop = { origin };
}

afterEach(() => {
	delete (window as unknown as { __wyndDesktop?: { origin: string } }).__wyndDesktop;
});

describe('desktop bridge', () => {
	it('без моста отдаёт адрес страницы', () => {
		expect(desktopBridge()).toBeUndefined();
		expect(pageOrigin()).toBe(window.location.origin);
	});

	it('с мостом отдаёт адрес сервера и считает его своим', () => {
		bridge('https://home.example.org');
		expect(desktopBridge()).toEqual({ origin: 'https://home.example.org' });
		expect(pageOrigin()).toBe('https://home.example.org');
		expect(normalizeOrigin('https://home.example.org')).toBe('');
		expect(resolveServerOrigin('home.example.org')).toBe('');
	});

	it('чужой сервер остаётся чужим', () => {
		bridge('https://home.example.org');
		expect(normalizeOrigin('https://other.example.org')).toBe('https://other.example.org');
		expect(resolveServerOrigin('https://other.example.org')).toBe('https://other.example.org');
	});

	it('чужой сервер идёт через /__srv', () => {
		bridge('https://home.example.org');
		expect(desktopProxyBase('https://other.example.org')).toBe('/__srv/https/other.example.org');
		expect(apiPath('https://other.example.org', '/instance')).toBe(
			'/__srv/https/other.example.org/api/v1/instance'
		);
		expect(apiPath('https://home.example.org', '/instance')).toBe('/api/v1/instance');
	});
});
