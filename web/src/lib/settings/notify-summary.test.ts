import { describe, expect, it } from 'vitest';
import { notifySummary } from './notify-mute';
import type { NotifyPrefs } from '$lib/circles/settings';

const base: NotifyPrefs = {
	posts: true,
	comments_mine: true,
	comments_all: false,
	reactions: false,
	mentions: true,
	events: false,
	mute_until: null
};
const day = () => '16 октября';

describe('notifySummary', () => {
	it('по умолчанию — записи, комментарии к моим и упоминания', () => {
		expect(notifySummary(base, day)).toBe('записи, комментарии к моим записям и упоминания');
	});
	it('всё выключено — только упоминания', () => {
		expect(notifySummary({ ...base, posts: false, comments_mine: false }, day)).toBe(
			'только упоминания'
		);
	});
	it('все комментарии заменяют «к моим»', () => {
		expect(notifySummary({ ...base, posts: false, comments_all: true, reactions: true }, day)).toBe(
			'все комментарии, реакции и упоминания'
		);
	});
	it('тишина — до какого дня', () => {
		const until = new Date(Date.now() + 3 * 86400 * 1000).toISOString();
		expect(notifySummary({ ...base, mute_until: until }, day)).toBe('выключены до 16 октября');
	});
});
