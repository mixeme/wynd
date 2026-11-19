import { describe, expect, it, vi } from 'vitest';
import { ApiError } from '$lib/api/client';
import type { FeedPost, FeedSnapshot } from '$lib/journal/types';
import { FeedData } from './feed-data.svelte';

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

function deps(over: Partial<ConstructorParameters<typeof FeedData>[0]> = {}) {
	return {
		load: async () => snapshot([]),
		loadOlder: async () => ({ items: [] as FeedPost[], next: undefined }),
		lastRead: () => 0,
		...over
	};
}

describe('FeedData', () => {
	it('ставит первую порцию, границу непрочитанного и отдаёт медиа экрана', async () => {
		const media: string[][] = [];
		const dayMedia: number[] = [];
		const feed = new FeedData(
			deps({
				load: async () => ({
					circle_id: 'c1',
					posts: [post('a', 5), post('b', 4)],
					events: [{ seq: 1, summary: 'день назван', created_at: '2026-11-17T09:00:00Z' }],
					visible_from: '2026-11-01T00:00:00Z',
					circle_started_at: '2026-10-01T00:00:00Z',
					next_before: 'older-1'
				}),
				lastRead: () => 4,
				resolveMedia: (items) => media.push(items.map((p) => p.id)),
				resolveDayMedia: (events) => dayMedia.push(events.length)
			})
		);

		await feed.load();

		expect(feed.loading).toBe(false);
		expect(feed.error).toBe('');
		expect(feed.posts.map((p) => p.id)).toEqual(['a', 'b']);
		expect(feed.nextBefore).toBe('older-1');
		// Граница «новое выше прочитанного»: первая запись новее 4, вторая — нет.
		expect(feed.dividerAt).toBe(1);
		expect(feed.feedEvents).toHaveLength(1);
		expect(feed.visibleFrom).toBe('2026-11-01T00:00:00Z');
		expect(feed.circleStartedAt).toBe('2026-10-01T00:00:00Z');
		expect(media).toEqual([['a', 'b']]);
		expect(dayMedia).toEqual([1]);
	});

	it('на ошибке показывает доступ или общую строку и снимает загрузку', async () => {
		const denied = new FeedData(
			deps({
				load: async () => {
					throw new ApiError(403, 'forbidden');
				}
			})
		);
		await denied.load();
		expect(denied.error).toBe('Нет доступа');
		expect(denied.loading).toBe(false);

		const broken = new FeedData(
			deps({
				load: async () => {
					throw new Error('network');
				}
			})
		);
		await broken.load();
		expect(broken.error).toBe('Не удалось загрузить ленту');
		expect(broken.loading).toBe(false);
	});

	it('при обновлении перечитывает догруженные старшие от новой границы', async () => {
		const loadOlder = vi.fn(async (before: string) => {
			if (before === 'cur-1') return { items: [post('c', 3)], next: 'cur-2' };
			if (before === 'new-1') return { items: [post('c', 3)], next: 'cur-3' };
			throw new Error('unexpected ' + before);
		});
		const snapshots = [snapshot([post('d', 4)], 'cur-1'), snapshot([post('e', 5)], 'new-1')];
		const feed = new FeedData(
			deps({
				load: async () => snapshots.shift()!,
				loadOlder
			})
		);

		await feed.load();
		await feed.loadOlder();
		expect(feed.olderPosts.map((p) => p.id)).toEqual(['c']);
		expect(feed.posts.map((p) => p.id)).toEqual(['d', 'c']);

		await feed.load();
		expect(loadOlder).toHaveBeenLastCalledWith('new-1');
		expect(feed.olderPosts.map((p) => p.id)).toEqual(['c']);
		expect(feed.nextBefore).toBe('cur-3');
		expect(feed.posts.map((p) => p.id)).toEqual(['e', 'c']);
	});

	it('догружает старшее только один раз и отдаёт на экран только новые записи', async () => {
		let finish!: (page: { items: FeedPost[]; next?: string }) => void;
		const loadOlder = vi.fn(
			() =>
				new Promise<{ items: FeedPost[]; next?: string }>((resolve) => {
					finish = resolve;
				})
		);
		const media: string[][] = [];
		const feed = new FeedData(
			deps({
				load: async () => snapshot([post('b', 2)], 'cur-1'),
				loadOlder,
				resolveMedia: (items) => media.push(items.map((p) => p.id))
			})
		);
		await feed.load();
		media.length = 0;

		const first = feed.loadOlder();
		expect(feed.loadingOlder).toBe(true);
		await feed.loadOlder();
		expect(loadOlder).toHaveBeenCalledTimes(1);

		finish({ items: [post('a', 1)], next: undefined });
		await first;
		expect(feed.loadingOlder).toBe(false);
		expect(feed.posts.map((p) => p.id)).toEqual(['b', 'a']);
		expect(feed.nextBefore).toBeUndefined();
		expect(media).toEqual([['a']]);

		await feed.loadOlder();
		expect(loadOlder).toHaveBeenCalledTimes(1);
	});

	it('молча переживает обрыв при догрузке', async () => {
		const feed = new FeedData(
			deps({
				load: async () => snapshot([post('b', 2)], 'cur-1'),
				loadOlder: async () => {
					throw new Error('offline');
				}
			})
		);
		await feed.load();
		await feed.loadOlder();
		expect(feed.posts.map((p) => p.id)).toEqual(['b']);
		expect(feed.loadingOlder).toBe(false);
		expect(feed.error).toBe('');
	});

	// C97: экран не ждёт сети, если снимок уже на устройстве.
	it('показывает снимок до ответа сети, сеть заменяет его', async () => {
		let answer!: (snap: FeedSnapshot) => void;
		const media: string[][] = [];
		const feed = new FeedData(
			deps({
				load: () =>
					new Promise<FeedSnapshot>((resolve) => {
						answer = resolve;
					}),
				loadCached: async () => snapshot([post('cached', 1)]),
				resolveMedia: (items) => media.push(items.map((p) => p.id))
			})
		);

		const network = feed.load();
		await feed.loadCached();
		expect(feed.posts.map((p) => p.id)).toEqual(['cached']);
		expect(feed.loading).toBe(false);
		expect(media).toEqual([['cached']]);

		answer(snapshot([post('fresh', 2)]));
		await network;
		expect(feed.posts.map((p) => p.id)).toEqual(['fresh']);
	});

	it('свежий ответ не перетирается опоздавшим снимком', async () => {
		let cached!: (snap: FeedSnapshot | undefined) => void;
		const feed = new FeedData(
			deps({
				load: async () => snapshot([post('fresh', 2)]),
				loadCached: () =>
					new Promise<FeedSnapshot | undefined>((resolve) => {
						cached = resolve;
					})
			})
		);

		await feed.load();
		const fromCache = feed.loadCached();
		cached(snapshot([post('stale', 1)]));
		await fromCache;
		expect(feed.posts.map((p) => p.id)).toEqual(['fresh']);
	});

	it('второй вызов снимка ждёт тот же, а не пустой ответ', async () => {
		let release!: (snap: FeedSnapshot | undefined) => void;
		const loadCached = vi.fn(
			() =>
				new Promise<FeedSnapshot | undefined>((resolve) => {
					release = resolve;
				})
		);
		const feed = new FeedData(deps({ loadCached }));

		let secondDone = false;
		const first = feed.loadCached();
		const second = feed.loadCached().then(() => {
			secondDone = true;
		});
		await Promise.resolve();
		expect(secondDone).toBe(false);

		release(snapshot([post('cached', 1)]));
		await Promise.all([first, second]);
		expect(feed.posts.map((p) => p.id)).toEqual(['cached']);
		expect(loadCached).toHaveBeenCalledTimes(1);
	});

	it('без снимка экран ждёт сеть, как прежде', async () => {
		const feed = new FeedData(
			deps({
				load: async () => snapshot([post('fresh', 1)]),
				loadCached: async () => undefined
			})
		);
		await feed.loadCached();
		expect(feed.posts).toEqual([]);
		expect(feed.loading).toBe(true);
		await feed.load();
		expect(feed.posts.map((p) => p.id)).toEqual(['fresh']);
		expect(feed.loading).toBe(false);
	});

	it('отражает свою реакцию только у выбранной записи', async () => {
		const feed = new FeedData(
			deps({
				load: async () => snapshot([post('a', 2), post('b', 1)], undefined)
			})
		);
		await feed.load();

		feed.applyReaction('b', 'heart', { identityId: 'me', identityName: 'Я' });
		const reactions = feed.posts[1].reactions ?? [];
		expect(reactions).toHaveLength(1);
		expect(reactions[0].emoji).toBe('heart');
		expect(reactions[0].identity_id).toBe('me');
		expect(feed.posts[0].reactions).toBeUndefined();

		feed.applyReaction('b', null, { identityId: 'me', identityName: 'Я' });
		expect(feed.posts[1].reactions).toHaveLength(0);
	});
});
