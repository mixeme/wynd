import { describe, expect, it } from 'vitest';
import { parseWyndLink } from './links';

describe('parseWyndLink', () => {
	it('parses circle invite paths', () => {
		expect(parseWyndLink('https://home.example.org/invite/abc123')).toBe('/invite/abc123');
		expect(parseWyndLink('/invite/abc123?members=1')).toBe('/invite/abc123?members=1');
	});

	it('parses server invite paths', () => {
		expect(parseWyndLink('https://home.example.org/join/xyz')).toBe('/join/xyz');
	});

	it('returns null for unrelated text', () => {
		expect(parseWyndLink('home.example.org')).toBeNull();
		expect(parseWyndLink('')).toBeNull();
	});
});
