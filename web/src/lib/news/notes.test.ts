import { beforeEach, describe, expect, it } from 'vitest';
import pkg from '../../../package.json';
import { NEWS } from './notes';
import { hasUnseenNews, latestNewsVersion, markNewsSeen } from './seen';

function minor(version: string): number[] {
	return version.split('.').slice(0, 2).map(Number);
}

function newer(a: string, b: string): boolean {
	const [a1, a2] = minor(a);
	const [b1, b2] = minor(b);
	return a1 !== b1 ? a1 > b1 : a2 > b2;
}

describe('записи «Что нового»', () => {
	it('идут от свежей к старой и не повторяются', () => {
		for (let i = 1; i < NEWS.length; i++) {
			expect(newer(NEWS[i - 1].version, NEWS[i].version)).toBe(true);
		}
	});

	it('начинаются с 0.13 и не забегают дальше версии приложения', () => {
		expect(NEWS[NEWS.length - 1].version).toBe('0.13');
		expect(newer(NEWS[0].version, pkg.version)).toBe(false);
	});

	it('у каждой записи есть день и хотя бы одна строка', () => {
		for (const entry of NEWS) {
			expect(entry.date).toMatch(/^\d{4}-\d{2}-\d{2}$/);
			expect(entry.items.length).toBeGreaterThan(0);
		}
	});
});

describe('отметка «видел»', () => {
	beforeEach(() => localStorage.clear());

	it('пока запись не открывали — она новая', () => {
		expect(hasUnseenNews()).toBe(true);
	});

	it('после открытия — уже нет, до следующей записи', () => {
		markNewsSeen();
		expect(hasUnseenNews()).toBe(false);
		localStorage.setItem('wynd:news-read', '0.1');
		expect(hasUnseenNews()).toBe(true);
		expect(latestNewsVersion()).toBe(NEWS[0].version);
	});
});
