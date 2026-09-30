import { describe, expect, it } from 'vitest';
import {
	groupResponses,
	responseHref,
	responseKindLabel,
	responsesDividerIndex,
	type ResponseItem
} from './responses';

function item(seq: number, patch: Partial<ResponseItem>): ResponseItem {
	return { kind: 'comment', seq, at: '2026-09-30T10:00:00Z', actor_id: 'x', actor_name: 'Кот', ...patch };
}

describe('отклики', () => {
	it('реакции подряд на одну запись — одна строка с именами', () => {
		const rows = groupResponses(
			[
				item(9, { kind: 'reaction', actor_name: 'Аня', post_id: 'p1', emoji: 'heart' }),
				item(8, { kind: 'reaction', actor_name: 'Петя', post_id: 'p1', emoji: 'heart' }),
				item(7, { kind: 'reaction', actor_name: 'Кот', post_id: 'p2', emoji: 'heart' }),
				item(6, { kind: 'comment', post_id: 'p1' })
			],
			0
		);
		expect(rows.map((r) => r.names)).toEqual([['Аня', 'Петя'], ['Кот'], ['Кот']]);
	});

	it('черта «выше — новое» не склеивает новое с прочитанным', () => {
		const rows = groupResponses(
			[
				item(9, { kind: 'reaction', actor_name: 'Аня', post_id: 'p1' }),
				item(5, { kind: 'reaction', actor_name: 'Петя', post_id: 'p1' }),
				item(4, { kind: 'comment' })
			],
			6
		);
		expect(rows).toHaveLength(3);
		expect(responsesDividerIndex(rows)).toBe(1);
		expect(responsesDividerIndex(groupResponses([item(3, {})], 6))).toBeNull();
	});

	it('подпись без глагола и адрес — туда, где отклик живёт', () => {
		expect(responseKindLabel('day_title')).toBe('название дня');
		expect(responseHref('c', item(1, { post_id: 'p', comment_id: 'm' }))).toBe(
			'/circles/c/posts/p?comment=m'
		);
		expect(responseHref('c', item(1, { kind: 'reaction', post_id: 'p' }))).toBe('/circles/c/posts/p');
		expect(responseHref('c', item(1, { kind: 'day_title', entry_date: '2026-08-06' }))).toBe(
			'/circles/c/days/2026-08-06'
		);
	});
});
