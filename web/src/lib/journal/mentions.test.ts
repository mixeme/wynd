import { describe, expect, it } from 'vitest';
import {
	filterMembersByMention,
	insertMention,
	mentionQueryAt,
	splitMentionBody,
	textByteLength
} from './mentions';

describe('mentionQueryAt', () => {
	it('detects query after @', () => {
		expect(mentionQueryAt('привет @Ан', 10)).toEqual({ start: 7, query: 'Ан' });
	});
	it('ignores @ in email', () => {
		expect(mentionQueryAt('you@example.com', 15)).toBeNull();
	});
});

describe('filterMembersByMention', () => {
	const members = [{ name: 'Аня' }, { name: 'Антон' }, { name: 'Кот' }];
	it('filters by prefix', () => {
		expect(filterMembersByMention(members, 'Ан').map((m) => m.name)).toEqual(['Аня', 'Антон']);
	});
});

describe('insertMention', () => {
	it('replaces partial query', () => {
		expect(insertMention('текст @Ан', 6, 9, 'Аня')).toBe('текст @Аня');
	});
});

describe('splitMentionBody', () => {
	it('splits mentions', () => {
		expect(splitMentionBody('@Аня, привет')).toEqual([
			{ kind: 'mention', value: '@Аня' },
			{ kind: 'text', value: ', привет' }
		]);
	});
});

describe('textByteLength', () => {
	it('counts utf-8 bytes', () => {
		expect(textByteLength('а')).toBe(2);
	});
});
