import { describe, expect, it } from 'vitest';
import { queueRetryDelayMs, readyQueueItems } from './queue';
import type { QueueRecordWithId } from '$lib/idb/db';

function item(over: Partial<QueueRecordWithId> & { id: number }): QueueRecordWithId {
	return {
		type: 'post',
		origin: 'https://a.example',
		circle_id: 'c1',
		payload: { body: 'текст', entry_date: '2026-08-30' },
		files: [],
		state: 'pending',
		created_at: 1000,
		...over
	} as QueueRecordWithId;
}

describe('queue backoff', () => {
	// Фиксированного повтора нет: недоступный сервер не должен опрашиваться
	// без пауз, но и ждать дольше четверти часа незачем (QUE-1).
	it('doubles from 30s up to 15 minutes', () => {
		expect(queueRetryDelayMs(1)).toBe(30_000);
		expect(queueRetryDelayMs(2)).toBe(60_000);
		expect(queueRetryDelayMs(5)).toBe(480_000);
		expect(queueRetryDelayMs(10)).toBe(900_000);
	});
});

describe('readyQueueItems', () => {
	it('keeps pending items and orders them by creation', () => {
		const ready = readyQueueItems(
			[item({ id: 2, created_at: 2000 }), item({ id: 1, created_at: 1000 })],
			5000
		);
		expect(ready.map((i) => i.id)).toEqual([1, 2]);
	});

	// Зависшая после падения вкладки отправка возвращается в работу, свежая —
	// нет: её сливает другая вкладка прямо сейчас.
	it('reclaims uploading older than two minutes, leaves fresh alone', () => {
		const now = 10 * 60 * 1000;
		const ready = readyQueueItems(
			[
				item({ id: 1, state: 'uploading', uploading_at: now - 5 * 60 * 1000 }),
				item({ id: 2, state: 'uploading', uploading_at: now - 10 * 1000 })
			],
			now
		);
		expect(ready.map((i) => i.id)).toEqual([1]);
	});

	it('waits out the backoff after a failed attempt', () => {
		const now = 1_000_000;
		const justTried = item({ id: 1, attempts: 2, uploading_at: now - 10_000 });
		const longAgo = item({ id: 2, attempts: 2, uploading_at: now - 5 * 60 * 1000 });
		expect(readyQueueItems([justTried, longAgo], now).map((i) => i.id)).toEqual([2]);
	});

	// 4xx кроме 401 оставляют запись в failed — её не надо трогать до правки.
	it('skips failed items', () => {
		expect(readyQueueItems([item({ id: 1, state: 'failed', error: 'invalid' })], 5000)).toEqual(
			[]
		);
	});
});
