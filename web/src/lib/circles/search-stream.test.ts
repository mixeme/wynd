import 'fake-indexeddb/auto';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { getDb, putSession } from '$lib/idb/db';
import { searchAllOrigins } from './circles';

// C95, «Поиск — позже»: ответ каждого сервера отдаётся немедленно, а не
// после всех. Медленный сервер больше не задерживает группу уже ответившего.
const A = 'http://a.test';
const B = 'http://b.test';

const HIT = {
	circle_id: 'c1',
	kind: 'post',
	post_id: 'p1',
	entry_date: '2026-11-19',
	created_at: '2026-11-19T10:00:00Z',
	author_name: 'Аня',
	snippet: 'Привет'
};

async function clear(): Promise<void> {
	const db = await getDb();
	for (const store of [
		'sessions',
		'snapshots',
		'settings',
		'groups',
		'pins',
		'street_ops'
	] as const) {
		await db.clear(store);
	}
}

async function session(origin: string, name: string): Promise<void> {
	await putSession({ origin, name, email: 'anya@example.com', token: 't', account_id: 'a' });
}

beforeEach(async () => {
	await clear();
	await session(A, 'Дом Ани');
	await session(B, 'Дача');
});

describe('searchAllOrigins — поток по серверам (C95)', () => {
	it('отдаёт ответ первого сервера до завершения всего поиска', async () => {
		let resolveB: (value: Response) => void = () => {};
		const bFlight = new Promise<Response>((resolve) => {
			resolveB = resolve;
		});
		const fetchStub = vi.fn((input: RequestInfo | URL) => {
			const url = String(input);
			if (url.startsWith(A)) {
				return Promise.resolve(new Response(JSON.stringify({ hits: [HIT] }), { status: 200 }));
			}
			return bFlight;
		});
		vi.stubGlobal('fetch', fetchStub);

		const seen: string[] = [];
		let done = false;
		const flight = searchAllOrigins('тест', undefined, 50, (origin) => {
			seen.push(origin);
		}).then((value) => {
			done = true;
			return value;
		});

		await vi.waitFor(() => expect(seen).toEqual([A]));
		// Второй сервер ещё не ответил — поиск в целом не завершён.
		expect(done).toBe(false);
		resolveB(new Response(JSON.stringify({ hits: [] }), { status: 200 }));
		await flight;
		expect(done).toBe(true);
		vi.unstubAllGlobals();
	});

	it('итог без onOrigin — прежняя форма', async () => {
		vi.stubGlobal(
			'fetch',
			vi.fn(() => Promise.resolve(new Response(JSON.stringify({ hits: [] }), { status: 200 })))
		);
		const { results, failedInstanceNames } = await searchAllOrigins('тест');
		expect(results).toEqual([]);
		expect(failedInstanceNames).toEqual([]);
		vi.unstubAllGlobals();
	});

	it('отказ сервера попадает в failedInstanceNames', async () => {
		vi.stubGlobal(
			'fetch',
			vi.fn((input: RequestInfo | URL) => {
				const url = String(input);
				if (url.startsWith(A)) {
					return Promise.resolve(new Response(JSON.stringify({ hits: [HIT] }), { status: 200 }));
				}
				return Promise.reject(new Error('offline'));
			})
		);
		const { results, failedInstanceNames } = await searchAllOrigins('тест');
		expect(results.map((r) => r.origin)).toEqual([A]);
		expect(failedInstanceNames).toEqual(['Дача']);
		vi.unstubAllGlobals();
	});
});
