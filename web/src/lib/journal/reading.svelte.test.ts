import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { FeedPost, FeedSnapshot } from '$lib/journal/types';
import { circleReading, dropCircleReading } from './reading.svelte';

vi.mock('$lib/media/batch', () => ({
	resolveMediaUrls: vi.fn(
		async (_origin: string, ids: string[], cb: (b: string, u: string) => void) => {
			for (const id of ids) cb(id, 'blob:' + id);
		}
	)
}));

vi.mock('./feed', () => ({
	loadFeed: vi.fn()
}));

import { loadFeed } from './feed';

const loadFeedMock = vi.mocked(loadFeed);

function post(id: string, seq: number, over: Partial<FeedPost> = {}): FeedPost {
	return {
		id,
		body: id,
		entry_date: '2026-11-17',
		author_name: 'Аня',
		identity_id: `i-${id}`,
		created_at: '2026-11-17T09:00:00Z',
		event_seq: seq,
		...over
	};
}

function snapshot(posts: FeedPost[], nextBefore?: string): FeedSnapshot {
	return { circle_id: 'c1', posts, next_before: nextBefore };
}

const KEY = ['', 'c1'] as const;

beforeEach(() => {
	loadFeedMock.mockReset();
	dropCircleReading(...KEY);
});

describe('чтение круга (C95-1)', () => {
	it('экран ленты и экран записи делят один контроллер без новой сети', async () => {
		loadFeedMock.mockResolvedValue(snapshot([post('a', 5)]));
		const feedScreen = circleReading(...KEY, () => 0);
		await feedScreen.feed.load();
		expect(feedScreen.feed.posts.map((p) => p.id)).toEqual(['a']);

		const postScreen = circleReading(...KEY, () => 0);
		expect(postScreen).toBe(feedScreen);
		// Запись открывается из загруженного: новой ленты не качали.
		expect(loadFeedMock).toHaveBeenCalledTimes(1);
		expect(postScreen.feed.posts.find((p) => p.id === 'a')).toBeDefined();
	});

	it('при возврате в ленту перенацеливает lastRead, медиа остаются общими', async () => {
		loadFeedMock.mockResolvedValue(
			snapshot([
				post('a', 5, {
					media: [{ blob_id: 'm1', kind: 'photo', is_cover: true }]
				}),
				post('b', 4)
			])
		);
		const feedScreen = circleReading(...KEY, () => 6);
		await feedScreen.feed.load();
		expect(feedScreen.feed.dividerAt).toBeNull();
		expect(feedScreen.mediaUrls['m1']).toBe('blob:m1');

		const restored = circleReading(...KEY, () => 4);
		expect(restored).toBe(feedScreen);
		expect(restored.mediaUrls['m1']).toBe('blob:m1');
		await restored.feed.load();
		expect(restored.feed.dividerAt).toBe(1);
	});

	it('уход со круга снимает чтение: новый экран читает заново', async () => {
		loadFeedMock.mockResolvedValue(snapshot([post('a', 5)]));
		const first = circleReading(...KEY, () => 0);
		await first.feed.load();

		dropCircleReading(...KEY);
		const second = circleReading(...KEY, () => 0);
		expect(second).not.toBe(first);
		await second.feed.load();
		expect(loadFeedMock).toHaveBeenCalledTimes(2);
	});
});
