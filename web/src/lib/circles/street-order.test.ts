import { describe, expect, it } from 'vitest';
import { sortStreetCircles, type StreetCircle } from './circles';

function row(name: string, patch: Partial<StreetCircle> = {}): StreetCircle {
	return {
		origin: '',
		instanceName: 'Wynd',
		id: name,
		name,
		unread: 0,
		pinned: false,
		pinnedAt: 0,
		color: '',
		initial: name[0],
		preview: '',
		time: '',
		...patch
	};
}

describe('порядок улицы', () => {
	it('закреплённые сверху, остальные по свежести, а не по имени', () => {
		const rows = [
			row('Архив', { lastAt: '2026-07-01T10:00:00Z' }),
			row('Дача', { lastAt: '2026-09-29T08:00:00Z' }),
			row('Семья', { lastAt: '2026-08-01T10:00:00Z', pinned: true, pinnedAt: 5 }),
			row('Ремонт', { lastAt: '2026-09-30T12:00:00Z' })
		];
		expect(sortStreetCircles(rows).map((r) => r.name)).toEqual(['Семья', 'Ремонт', 'Дача', 'Архив']);
	});

	it('приглашение без имени — самое свежее, круг без событий — в конце', () => {
		const rows = [
			row('Без событий'),
			row('Дача', { lastAt: '2026-09-29T08:00:00Z' }),
			row('Позвали', { pendingJoin: true })
		];
		expect(sortStreetCircles(rows).map((r) => r.name)).toEqual(['Позвали', 'Дача', 'Без событий']);
	});
});
