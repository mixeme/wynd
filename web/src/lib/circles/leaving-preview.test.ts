import { beforeEach, expect, it, vi } from 'vitest';
import { apiJson } from '$lib/api/client';
import { getDb, putSession } from '$lib/idb/db';
import { loadStreetCircles } from './circles';

vi.mock('$lib/api/client', async (original) => ({
	...(await original<typeof import('$lib/api/client')>()),
	apiJson: vi.fn()
}));

beforeEach(async () => {
	vi.mocked(apiJson).mockReset();
	const db = await getDb();
	await db.clear('sessions');
	await putSession({
		origin: '',
		name: 'Дома Ани',
		email: 'anya@example.org',
		token: 'test',
		account_id: 'a1'
	});
});

it('после приёма круга напоминает владельцу завершить перенос, участнику — войти на новый сервер', async () => {
	const base = {
		name: 'Семья',
		status: 'active',
		color: 'terracotta',
		frozen_at: '2026-11-02T12:00:00Z'
	};
	vi.mocked(apiJson).mockImplementation(async (_origin, path) => {
		if (path === '/pending-circle-joins') return { circle_ids: [] };
		if (path === '/circles')
			return {
				circles: [
					{
						...base,
						id: 'owner',
						you_own: true,
						move: { status: 'accepted', dest_url: 'https://next.example' }
					},
					{
						...base,
						id: 'participant',
						you_own: false,
						move: { status: 'accepted', dest_url: 'https://next.example' }
					},
					{ ...base, id: 'moving', you_own: true, move: { status: 'pulling' } }
				]
			};
		throw new Error(`Unexpected request: ${path}`);
	});
	const circles = await loadStreetCircles();
	expect(circles.find((c) => c.id === 'owner')?.preview).toBe('Круг переехал — завершите перенос');
	expect(circles.find((c) => c.id === 'participant')?.preview).toBe(
		'Круг переехал — войдите на next.example'
	);
	expect(circles.find((c) => c.id === 'moving')?.preview).toBe('Круг замер на время переезда');
});
