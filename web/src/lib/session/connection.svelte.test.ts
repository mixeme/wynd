import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { QueueRecordWithId } from '$lib/idb/db';

// Очередь — подставная: модуль связи только читает её и просит слить.
const queue = vi.hoisted(() => ({
	items: [] as unknown[],
	listeners: new Set<() => void>(),
	progress: new Set<(id: number, fraction: number | undefined) => void>(),
	drains: [] as Array<{ origin: string; finish: () => void }>
}));

vi.mock('$lib/queue/queue', () => ({
	listQueueItems: async () => queue.items,
	subscribeQueue: (fn: () => void) => {
		queue.listeners.add(fn);
		return () => queue.listeners.delete(fn);
	},
	subscribeQueueProgress: (fn: (id: number, fraction: number | undefined) => void) => {
		queue.progress.add(fn);
		return () => queue.progress.delete(fn);
	},
	drainQueueFor: (origin: string) =>
		new Promise<void>((resolve) => queue.drains.push({ origin, finish: resolve }))
}));

import {
	OFFLINE_GRACE_MS,
	SERVER_GRACE_MS,
	connectionNotice,
	forgetServer,
	initConnection,
	isServerDown,
	reportServerSilent,
	reportServerUp
} from './connection.svelte';

const A = 'https://a.example';
const B = 'https://b.example';

function item(id: number, over: Partial<QueueRecordWithId> = {}): QueueRecordWithId {
	return {
		id,
		type: 'post',
		origin: A,
		circle_id: 'c1',
		payload: { body: 'текст', entry_date: '2026-08-30' },
		files: [],
		state: 'pending',
		created_at: id,
		...over
	} as QueueRecordWithId;
}

let online = true;
let stop: (() => void) | undefined;

function setOnline(value: boolean) {
	online = value;
	window.dispatchEvent(new Event(value ? 'online' : 'offline'));
}

async function settle() {
	await vi.advanceTimersByTimeAsync(0);
}

beforeEach(() => {
	vi.useFakeTimers();
	online = true;
	vi.spyOn(navigator, 'onLine', 'get').mockImplementation(() => online);
	queue.items = [];
	queue.drains = [];
	stop = initConnection();
});

afterEach(async () => {
	// Состояние модуля общее на файл: вернуть «на связи» перед следующим тестом.
	setOnline(true);
	for (const origin of [A, B]) forgetServer(origin);
	for (const d of queue.drains) d.finish();
	await settle();
	stop?.();
	vi.restoreAllMocks();
	vi.useRealTimers();
});

describe('нет сети (7.6)', () => {
	it('короткий обрыв не мигает', async () => {
		setOnline(false);
		await vi.advanceTimersByTimeAsync(OFFLINE_GRACE_MS - 500);
		expect(connectionNotice()).toBeUndefined();
		setOnline(true);
		await vi.advanceTimersByTimeAsync(OFFLINE_GRACE_MS);
		expect(connectionNotice()).toBeUndefined();
	});

	it('дольше срока — полоса и в круге, и на улочке', async () => {
		setOnline(false);
		await vi.advanceTimersByTimeAsync(OFFLINE_GRACE_MS);
		expect(connectionNotice()).toEqual({ kind: 'offline' });
		expect(connectionNotice(A)).toEqual({ kind: 'offline' });
	});

	// Без сети молчат все серверы: полоса про сеть, а не про каждый.
	it('без сети сервер молчащим не считается', async () => {
		setOnline(false);
		reportServerSilent(A);
		await vi.advanceTimersByTimeAsync(SERVER_GRACE_MS * 2);
		expect(isServerDown(A)).toBe(false);
	});

	it('связь вернулась, ждать нечего — полоса просто пропадает', async () => {
		setOnline(false);
		await vi.advanceTimersByTimeAsync(OFFLINE_GRACE_MS);
		setOnline(true);
		await settle();
		expect(connectionNotice(A)).toBeUndefined();
	});
});

describe('сервер не отвечает (7.7, 7.9)', () => {
	it('молчит дольше срока — полоса у его кругов, у чужих нет', async () => {
		reportServerSilent(A);
		await vi.advanceTimersByTimeAsync(SERVER_GRACE_MS - 500);
		expect(isServerDown(A)).toBe(false);
		await vi.advanceTimersByTimeAsync(500);
		expect(connectionNotice(A)).toEqual({ kind: 'down' });
		expect(connectionNotice(B)).toBeUndefined();
		// На улочке общей полосы нет — сказано у кругов.
		expect(connectionNotice()).toBeUndefined();
	});

	it('ответил до срока — полосы не было', async () => {
		reportServerSilent(A);
		await vi.advanceTimersByTimeAsync(SERVER_GRACE_MS - 500);
		reportServerUp(A);
		await vi.advanceTimersByTimeAsync(SERVER_GRACE_MS);
		expect(isServerDown(A)).toBe(false);
	});

	// Повторные неудачи срок не продлевают: иначе при повторах чаще срока
	// полоса не встала бы никогда.
	it('повторная неудача не откладывает полосу', async () => {
		reportServerSilent(A);
		await vi.advanceTimersByTimeAsync(SERVER_GRACE_MS - 1000);
		reportServerSilent(A);
		await vi.advanceTimersByTimeAsync(1000);
		expect(isServerDown(A)).toBe(true);
	});

	it('сеть пропала — вместо «сервер молчит» говорим про сеть', async () => {
		reportServerSilent(A);
		await vi.advanceTimersByTimeAsync(SERVER_GRACE_MS);
		setOnline(false);
		await vi.advanceTimersByTimeAsync(OFFLINE_GRACE_MS);
		expect(connectionNotice(A)).toEqual({ kind: 'offline' });
		expect(isServerDown(A)).toBe(false);
	});
});

describe('связь вернулась (7.8)', () => {
	it('сервер ожил, в очереди ждёт — «отправляем» с ходом, потом тишина', async () => {
		queue.items = [item(1), item(2, { type: 'comment' }), item(3, { origin: B })];
		reportServerSilent(A);
		await vi.advanceTimersByTimeAsync(SERVER_GRACE_MS);
		reportServerUp(A);
		await settle();
		expect(connectionNotice(A)).toEqual({ kind: 'sending', posts: 1, comments: 1, progress: 0 });
		// Чужой сервер не молчал — про его очередь полоса не говорит.
		expect(connectionNotice(B)).toBeUndefined();
		expect(queue.drains.map((d) => d.origin)).toEqual([A]);

		// Половина первой записи ушла.
		for (const fn of queue.progress) fn(1, 0.5);
		await settle();
		expect(connectionNotice(A)).toMatchObject({ progress: 0.25 });

		// Первая ушла целиком.
		queue.items = [item(2, { type: 'comment' }), item(3, { origin: B })];
		for (const fn of queue.progress) fn(1, undefined);
		for (const fn of queue.listeners) fn();
		await settle();
		expect(connectionNotice(A)).toEqual({ kind: 'sending', posts: 0, comments: 1, progress: 0.5 });

		queue.items = [item(3, { origin: B })];
		for (const fn of queue.listeners) fn();
		await settle();
		expect(connectionNotice(A)).toBeUndefined();
	});

	// Отказ по существу (4xx) в очереди — не «ждёт отправки»; реакции не считаем.
	it('отклонённое и реакции в счёт не идут', async () => {
		queue.items = [item(1, { state: 'failed' }), item(2, { type: 'reaction' })];
		reportServerSilent(A);
		await vi.advanceTimersByTimeAsync(SERVER_GRACE_MS);
		reportServerUp(A);
		await settle();
		expect(connectionNotice(A)).toBeUndefined();
	});

	// Слив кончился, а запись осталась ждать повтора — полоса не висит.
	it('слив закончился — полоса уходит, даже если что-то осталось', async () => {
		queue.items = [item(1)];
		setOnline(false);
		await vi.advanceTimersByTimeAsync(OFFLINE_GRACE_MS);
		setOnline(true);
		await settle();
		expect(connectionNotice(A)).toMatchObject({ kind: 'sending', posts: 1 });
		queue.drains[0].finish();
		await settle();
		expect(connectionNotice(A)).toBeUndefined();
	});
});
