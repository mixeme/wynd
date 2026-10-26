import 'fake-indexeddb/auto';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { getDb, listGroups, listStreetOps, putGroup, putPin } from '$lib/idb/db';
import {
	applyStreetOps,
	applyStreetOpToView,
	flushStreetOps,
	mergeStreetGroups,
	moveDeviceStreetToServers,
	queueStreetOp,
	type StreetLayout
} from './street';

const A = 'https://a.example';
const B = 'https://b.example';

function layout(groups: StreetLayout['groups'], pins: Array<[string, number]> = []): StreetLayout {
	return { pins: new Map(pins), groups };
}

describe('applyStreetOps', () => {
	// Неотправленное лежит поверх ответа сервера так, как его сделает сервер.
	it('moves a circle between groups and drops the emptied one', () => {
		const out = applyStreetOps(layout([{ name: 'Дача', circle_ids: ['c1'] }]), [
			{ origin: A, op: 'group', circle_id: 'c1', name: 'Семья' }
		]);
		expect(out.groups).toEqual([{ name: 'Семья', circle_ids: ['c1'] }]);
	});

	it('merges groups when renamed into a taken name', () => {
		const out = applyStreetOps(
			layout([
				{ name: 'Дача', circle_ids: ['c1'] },
				{ name: 'Семья', circle_ids: ['c2'] }
			]),
			[{ origin: A, op: 'rename', name: 'Дача', new_name: 'Семья' }]
		);
		expect(out.groups).toEqual([{ name: 'Семья', circle_ids: ['c2', 'c1'] }]);
	});

	it('pins, unpins, ungroups and drops', () => {
		const out = applyStreetOps(
			layout(
				[
					{ name: 'Дача', circle_ids: ['c1', 'c2'] },
					{ name: 'Работа', circle_ids: ['c3'] }
				],
				[['c1', 5]]
			),
			[
				{ origin: A, op: 'unpin', circle_id: 'c1' },
				{ origin: A, op: 'pin', circle_id: 'c2', pinned_at: 7 },
				{ origin: A, op: 'ungroup', circle_id: 'c1' },
				{ origin: A, op: 'drop', name: 'Работа' }
			]
		);
		expect([...out.pins]).toEqual([['c2', 7]]);
		expect(out.groups).toEqual([{ name: 'Дача', circle_ids: ['c2'] }]);
	});
});

describe('mergeStreetGroups', () => {
	// Группы серверов складываются по названию, по алфавиту; запись устройства
	// даёт свёрнутость; пустая новая — видна, опустевшая — уходит.
	it('joins servers by name and settles device records', () => {
		const fresh = { id: 'l1', name: 'Новая', circleIds: [], collapsed: false };
		const emptied = { id: 'l2', name: 'Была', circleIds: [], collapsed: false, filled: true };
		const folded = { id: 'l3', name: 'Семья', circleIds: [], collapsed: true };
		const out = mergeStreetGroups(
			[
				{ origin: A, groups: [{ name: 'Семья', circle_ids: ['c1'] }] },
				{
					origin: B,
					groups: [
						{ name: 'Семья', circle_ids: ['c9'] },
						{ name: 'Дача', circle_ids: ['c8'] }
					]
				}
			],
			[fresh, emptied, folded]
		);
		expect(out.groups).toEqual([
			{ name: 'Дача', keys: [`${B}:c8`], collapsed: false },
			{ name: 'Новая', keys: [], collapsed: false },
			{ name: 'Семья', keys: [`${A}:c1`, `${B}:c9`], collapsed: true }
		]);
		expect(out.drop).toEqual([emptied]);
		expect(out.fill).toEqual([folded]);
	});
});

describe('applyStreetOpToView', () => {
	const view = [
		{ name: 'Дача', keys: [`${A}:c1`], collapsed: true },
		{ name: 'Новая', keys: [], collapsed: false },
		{ name: 'Семья', keys: [`${B}:c2`], collapsed: false }
	];

	// Экран меняется сразу, как изменится после ответа серверов: опустевшая
	// группа уходит, а пустая новая остаётся, пока в неё не положили круг.
	it('moves a circle into the fresh empty group and drops the emptied one', () => {
		expect(
			applyStreetOpToView(view, { origin: A, op: 'group', circle_id: 'c1', name: 'Новая' })
		).toEqual([
			{ name: 'Новая', keys: [`${A}:c1`], collapsed: false },
			{ name: 'Семья', keys: [`${B}:c2`], collapsed: false }
		]);
	});

	it('renames into a taken name by merging, and drops', () => {
		expect(
			applyStreetOpToView(view, { origin: A, op: 'rename', name: 'Дача', new_name: 'Семья' })
		).toEqual([
			{ name: 'Новая', keys: [], collapsed: false },
			{ name: 'Семья', keys: [`${B}:c2`, `${A}:c1`], collapsed: false }
		]);
		expect(
			applyStreetOpToView(view, { origin: B, op: 'drop', name: 'Семья' }).map((g) => g.name)
		).toEqual(['Дача', 'Новая']);
	});
});

describe('street on the device', () => {
	beforeEach(async () => {
		const db = await getDb();
		for (const store of ['pins', 'groups', 'street_ops'] as const) await db.clear(store);
	});
	afterEach(() => vi.unstubAllGlobals());

	// Раскладка до A2 уходит на серверы один раз; круги сервера, где человек
	// уже не вошёл, отбрасываются; на устройстве остаётся свёрнутость.
	it('moves device pins and groups into the queue', async () => {
		await putPin(A, 'c1');
		await putPin('https://gone.example', 'c5');
		await putGroup({ id: 'g1', name: 'Семья', circleIds: [`${A}:c2`, `${B}:c3`], collapsed: true });

		await moveDeviceStreetToServers([A, B]);
		await moveDeviceStreetToServers([A, B]);

		const ops = await listStreetOps();
		expect(ops.map(({ origin, op, circle_id, name }) => ({ origin, op, circle_id, name }))).toEqual(
			[
				{ origin: A, op: 'pin', circle_id: 'c1', name: undefined },
				{ origin: A, op: 'group', circle_id: 'c2', name: 'Семья' },
				{ origin: B, op: 'group', circle_id: 'c3', name: 'Семья' }
			]
		);
		expect(ops[0].pinned_at).toBeGreaterThan(0);
		expect(await listGroups()).toEqual([
			{ id: 'g1', name: 'Семья', circleIds: [], collapsed: true, filled: true }
		]);
	});

	// Молчащий сервер получит изменения, когда ответит; отказ по существу
	// снимает изменение — повтор ответа не изменит.
	it('keeps ops while the server is silent and drops refused ones', async () => {
		await queueStreetOp({ origin: A, op: 'group', circle_id: 'gone', name: 'Семья' });
		await queueStreetOp({ origin: A, op: 'pin', circle_id: 'c1', pinned_at: 1 });
		await queueStreetOp({ origin: B, op: 'unpin', circle_id: 'c2' });

		vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new TypeError('Failed to fetch')));
		await flushStreetOps(A);
		expect((await listStreetOps()).length).toBe(3);

		const calls: string[] = [];
		vi.stubGlobal(
			'fetch',
			vi.fn(async (url: string, init: RequestInit) => {
				calls.push(`${init.method} ${url} ${init.body ?? ''}`);
				if (url.includes('/gone/')) {
					return new Response(JSON.stringify({ error: 'not_found' }), { status: 404 });
				}
				return new Response('{"status":"ok"}', { status: 200 });
			})
		);
		await flushStreetOps(A);
		expect(calls).toEqual([
			`PUT ${A}/api/v1/circles/gone/group {"name":"Семья"}`,
			`PUT ${A}/api/v1/circles/c1/pin {"pinned_at":"1970-01-01T00:00:00.001Z"}`
		]);
		expect((await listStreetOps()).map((op) => op.origin)).toEqual([B]);
	});
});
