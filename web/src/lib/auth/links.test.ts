import { describe, expect, it } from 'vitest';
import { foreignWyndLinkOrigin, inviteTarget, parseWyndLink } from './links';

describe('parseWyndLink', () => {
	const here = 'https://home.example.org';

	it('parses circle invite paths', () => {
		expect(parseWyndLink('https://home.example.org/invite/abc123', here)).toBe('/invite/abc123');
		expect(parseWyndLink('/invite/abc123?members=1', here)).toBe('/invite/abc123?members=1');
	});

	it('parses server invite paths', () => {
		expect(parseWyndLink('https://home.example.org/join/xyz', here)).toBe('/join/xyz');
	});

	// Аудит 2026-09-22: ссылка на другой сервер не разбирается — её токен
	// иначе уходил на текущий сервер и оседал в его логах.
	it('rejects links to another server', () => {
		expect(parseWyndLink('https://other.example.org/invite/abc123', here)).toBeNull();
		expect(parseWyndLink('https://home.example.org:8443/invite/abc123', here)).toBeNull();
		expect(foreignWyndLinkOrigin('https://other.example.org/invite/abc123', here)).toBe(
			'https://other.example.org'
		);
		expect(foreignWyndLinkOrigin('https://home.example.org/invite/abc123', here)).toBeNull();
		expect(foreignWyndLinkOrigin('/invite/abc123', here)).toBeNull();
	});

	it('returns null for unrelated text', () => {
		expect(parseWyndLink('home.example.org', here)).toBeNull();
		expect(parseWyndLink('', here)).toBeNull();
	});
});

// Один разбор для вставки и для сканера: путь приглашения или отказ словами.
describe('inviteTarget', () => {
	const here = 'https://wynd.example';
	it('opens an invite of this server', () => {
		expect(inviteTarget(`${here}/invite/abc`, here)).toEqual({ path: '/invite/abc' });
	});
	it('refuses a link to another server without sending its token here', () => {
		const t = inviteTarget('https://other.example/invite/abc', here);
		expect('error' in t && t.error).toContain('другой сервер');
	});
	it('refuses plain text', () => {
		expect(inviteTarget('привет', here)).toEqual({ error: 'Это не ссылка-приглашение Wynd' });
	});
});

// Ссылка подключения устройства (план 72): «/?email=почта» того же сервера.
describe('parseWyndLink: login link', () => {
	const here = 'https://wynd.example';
	it('opens the login screen with the email from the link', () => {
		expect(parseWyndLink(`${here}/?email=anya%40example.com`, here)).toBe(
			'/?email=anya%40example.com'
		);
	});
	it('keeps other query parameters', () => {
		expect(parseWyndLink(`${here}/?email=a@b.c&x=1`, here)).toBe('/?email=a@b.c&x=1');
	});
	it('ignores the root without email', () => {
		expect(parseWyndLink(`${here}/`, here)).toBeNull();
	});
	it('refuses a login link to another server', () => {
		expect(parseWyndLink('https://other.example/?email=a@b.c', here)).toBeNull();
	});
});
