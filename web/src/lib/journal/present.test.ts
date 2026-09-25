import { describe, expect, it } from 'vitest';
import {
	commentPreview,
	groupReactions,
	reactionIconName,
	unreadDividerIndex
} from './present';
import type { Comment, FeedPost, Reaction } from './types';

describe('present', () => {
	it('groups reactions by emoji preserving first-seen order', () => {
		const reactions: Reaction[] = [
			{ emoji: 'heart', author_name: 'Аня', identity_id: 'a' },
			{ emoji: 'laugh', author_name: 'Боб', identity_id: 'b' },
			{ emoji: 'heart', author_name: 'Вера', identity_id: 'c' }
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
			{ author_name: 'Аня', body: 'привет', identity_id: 'a' },
			{ author_name: 'Боб', body: 'ответ', identity_id: 'b' },
			{ author_name: 'Вера', body: 'ещё', identity_id: 'c' }
		];
		expect(commentPreview(comments)).toEqual({
			first: 'Аня: привет',
			more: 2
		});
		expect(commentPreview([])).toEqual({ more: 0 });
	});

	it('places unread divider before first read post', () => {
		const posts: FeedPost[] = [
			{ id: 'p3', event_seq: 30, body: '' },
			{ id: 'p2', event_seq: 20, body: '' },
			{ id: 'p1', event_seq: 10, body: '' }
		];
		expect(unreadDividerIndex(posts, 15)).toBe(2);
		expect(unreadDividerIndex(posts, 0)).toBeNull();
		expect(unreadDividerIndex(posts, 30)).toBeNull();
		expect(unreadDividerIndex([{ id: 'p', event_seq: 5, body: '' }], 0)).toBeNull();
		expect(unreadDividerIndex([{ id: 'p', event_seq: 5, body: '' }], 1)).toBe(1);
	});
});
