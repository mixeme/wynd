import { beforeEach, describe, expect, it, vi } from 'vitest';
import { fetchCircles, fetchPendingCircleJoins, loadCirclesCached } from '$lib/circles/circles';
import { listSessions } from '$lib/idb/db';
import { resolveCircleOrigin } from './origin';

vi.mock('$lib/idb/db', async (original) => ({
	...(await original<typeof import('$lib/idb/db')>()),
	listSessions: vi.fn()
}));
vi.mock('$lib/circles/circles', async (original) => ({
	...(await original<typeof import('$lib/circles/circles')>()),
	loadCirclesCached: vi.fn(),
	fetchCircles: vi.fn(),
	fetchPendingCircleJoins: vi.fn()
}));

beforeEach(() => {
	vi.mocked(listSessions).mockResolvedValue([
		{ origin: 'https://s1.example.org', email: 'a@b.example' } as never
	]);
	vi.mocked(loadCirclesCached).mockResolvedValue([]);
	vi.mocked(fetchCircles).mockResolvedValue([]);
	vi.mocked(fetchPendingCircleJoins).mockResolvedValue([]);
	sessionStorage.clear();
});

describe('resolveCircleOrigin (C104)', () => {
	it('круг есть в снимке — сервер определяется без сети', async () => {
		vi.mocked(loadCirclesCached).mockResolvedValue([
			{ id: 'c1', name: 'Опал', status: 'active' } as never
		]);
		await expect(resolveCircleOrigin('c1')).resolves.toBe('https://s1.example.org');
		expect(fetchCircles).not.toHaveBeenCalled();
	});

	it('в снимках нет — спрашиваем сеть, как раньше', async () => {
		vi.mocked(fetchCircles).mockResolvedValue([
			{ id: 'c2', name: 'Другой круг', status: 'active' } as never
		]);
		await expect(resolveCircleOrigin('c1')).resolves.toBeNull();
		expect(fetchCircles).toHaveBeenCalled();
		expect(fetchPendingCircleJoins).toHaveBeenCalled();
	});
});
