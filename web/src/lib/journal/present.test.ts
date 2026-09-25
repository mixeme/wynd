import { describe, expect, it } from 'vitest';
import {
	albumDownloadFilename,
	commentPreview,
	groupReactions,
	reactionIconName,
	serviceEventsAboveNewest,
	serviceEventsBetween,
	unreadDividerIndex
} from './present';
import type { Comment, FeedPost, Reaction } from './types';

const at = '2026-01-01T00:00:00Z';

describe('present', () => {
	it('groups reactions by emoji preserving first-seen order', () => {
		const reactions: Reaction[] = [
			{ id: 'r1', post_id: 'p', emoji: 'heart', author_name: 'Аня', identity_id: 'a', created_at: at },
			{ id: 'r2', post_id: 'p', emoji: 'laugh', author_name: 'Боб', identity_id: 'b', created_at: at },
			{ id: 'r3', post_id: 'p', emoji: 'heart', author_name: 'Вера', identity_id: 'c', created_at: at }
		];
		expect(groupReactions(reactions)).toEqual([
			{ emoji: 'heart', names: 'Аня, Вера' },
			{ emoji: 'laugh', names: 'Боб' }
		]);
	});

	it('maps reaction emoji to icon name', () => {
		expect(reactionIconName('laugh')).toBe('laugh');
		expect(reactionIconName('surprise')).toBe('surprise');
		expect(reactionIconName('anger')).toBe('anger');
		expect(reactionIconName('heart')).toBe('heart');
		expect(reactionIconName('unknown')).toBe('heart');
	});

	it('builds comment preview with more count', () => {
		const comments: Comment[] = [
			{ id: 'c1', post_id: 'p', author_name: 'Аня', body: 'привет', identity_id: 'a', created_at: at },
			{ id: 'c2', post_id: 'p', author_name: 'Боб', body: 'ответ', identity_id: 'b', created_at: at },
			{ id: 'c3', post_id: 'p', author_name: 'Вера', body: 'ещё', identity_id: 'c', created_at: at }
		];
		expect(commentPreview(comments)).toEqual({
			first: 'Аня: привет',
			createdAt: at,
			more: 2
		});
		expect(commentPreview([])).toEqual({ more: 0 });
	});

	it('places unread divider before first read post', () => {
		const posts: FeedPost[] = [
			{ id: 'p3', event_seq: 30, body: '', entry_date: '2026-01-01', author_name: '', identity_id: '', created_at: at },
			{ id: 'p2', event_seq: 20, body: '', entry_date: '2026-01-01', author_name: '', identity_id: '', created_at: at },
			{ id: 'p1', event_seq: 10, body: '', entry_date: '2026-01-01', author_name: '', identity_id: '', created_at: at }
		];
		expect(unreadDividerIndex(posts, 15)).toBe(2);
		expect(unreadDividerIndex(posts, 0)).toBeNull();
		expect(unreadDividerIndex(posts, 30)).toBeNull();
		expect(
			unreadDividerIndex(
				[{ id: 'p', event_seq: 5, body: '', entry_date: '2026-01-01', author_name: '', identity_id: '', created_at: at }],
				0
			)
		).toBeNull();
		expect(
			unreadDividerIndex(
				[{ id: 'p', event_seq: 5, body: '', entry_date: '2026-01-01', author_name: '', identity_id: '', created_at: at }],
				1
			)
		).toBe(1);
	});

	it('names lightbox downloads from filename or kind and 1-based index', () => {
		expect(
			albumDownloadFilename(
				{ blob_id: 'a', kind: 'photo', is_cover: false, filename: ' IMG.JPG ' },
				0
			)
		).toBe('IMG.JPG');
		expect(albumDownloadFilename({ blob_id: 'a', kind: 'photo', is_cover: false }, 0)).toBe(
			'photo-1.jpg'
		);
		expect(albumDownloadFilename({ blob_id: 'b', kind: 'video', is_cover: false }, 2)).toBe(
			'video-3.mp4'
		);
	});

	it('shows service events above the newest post and when feed is empty', () => {
		const events = [
			{ seq: 5, summary: 'Мышка теперь Мышь', created_at: at },
			{ seq: 3, summary: 'Боб вступил', created_at: at }
		];
		expect(serviceEventsAboveNewest(events, 4).map((e) => e.seq)).toEqual([5]);
		expect(serviceEventsAboveNewest(events, 5).map((e) => e.seq)).toEqual([]);
		expect(serviceEventsAboveNewest(events, undefined).map((e) => e.seq)).toEqual([5, 3]);
		expect(serviceEventsBetween(events, 10, 0).map((e) => e.seq)).toEqual([5, 3]);
	});
});
