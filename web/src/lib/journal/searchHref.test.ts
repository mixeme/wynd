import { describe, expect, it } from 'vitest';
import { searchHitHref } from './search';

describe('searchHitHref', () => {
	const base = { post_id: 'p', entry_date: '2026-10-01' };
	it('ведёт к найденному и просит подсветить', () => {
		expect(searchHitHref('c', { ...base, kind: 'day' })).toBe('/circles/c/days/2026-10-01');
		expect(searchHitHref('c', { ...base, kind: 'comment', comment_id: 'm' })).toBe('/circles/c/posts/p?comment=m');
		expect(searchHitHref('c', { ...base, kind: 'audio', media_blob_id: 'b' })).toBe('/circles/c/posts/p?media=b');
		expect(searchHitHref('c', { ...base, kind: 'file', media_blob_id: 'b' })).toBe('/circles/c/posts/p?media=b');
		expect(searchHitHref('c', { ...base, kind: 'post' })).toBe('/circles/c/posts/p?found=1');
	});
});
