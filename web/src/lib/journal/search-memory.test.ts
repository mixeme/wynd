import { beforeEach, expect, it } from 'vitest';
import {
	forgetSearchScreens,
	recallSearchScreen,
	rememberSearchScreen,
	searchScreenKey
} from './search-memory';
import type { CircleSearchHit } from './types';

function hit(postId: string, snippet = 'на даче'): CircleSearchHit {
	return {
		post_id: postId,
		circle_id: 'one',
		kind: 'post',
		snippet,
		entry_date: '2026-10-04',
		created_at: '2026-10-04T11:27:00Z',
		author_name: 'Ольга',
		thumb_blob_id: 'blob-' + postId
	};
}

const path = '/circles/one/search';
const key = searchScreenKey('https://wynd.example', 'one', path, { q: 'дач', hasPhoto: true });

beforeEach(() => forgetSearchScreens());

it('ключ — круг, сервер и канонический адрес поиска', () => {
	expect(key).toBe(
		searchScreenKey('https://wynd.example/', 'one', path, { q: ' дач ', hasPhoto: true })
	);
	expect(key).not.toBe(searchScreenKey('https://wynd.example', 'two', path, { q: 'дач' }));
	expect(key).not.toBe(searchScreenKey('', 'one', path, { q: 'дач' }));
	expect(key).toContain('q=%D0%B4%D0%B0%D1%87');
	expect(key).toContain('photo=1');
});

it('экран возвращается целиком: строки, авторы, превью и «Ищем…»', () => {
	rememberSearchScreen(key, {
		hits: [hit('p1')],
		authors: ['Ольга', 'Микс'],
		thumbs: { p1: 'blob:one' },
		loading: false
	});
	expect(recallSearchScreen(key)).toEqual({
		key,
		hits: [hit('p1')],
		authors: ['Ольга', 'Микс'],
		thumbs: { p1: 'blob:one' },
		loading: false
	});
});

it('чужой запрос и другой круг не получают чужие строки', () => {
	rememberSearchScreen(key, { hits: [hit('p1')], authors: [], thumbs: {}, loading: false });
	expect(
		recallSearchScreen(searchScreenKey('https://wynd.example', 'one', path, { q: 'дач' }))
	).toBeUndefined();
	expect(
		recallSearchScreen(searchScreenKey('https://wynd.example', 'two', path, { q: 'дач' }))
	).toBeUndefined();
});

it('держит несколько последних экранов и вытесняет давно не открытый', () => {
	const keys = Array.from({ length: 7 }, (_, i) =>
		searchScreenKey('https://wynd.example', 'one', path, { q: 'запрос' + i })
	);
	for (const k of keys) {
		rememberSearchScreen(k, { hits: [hit('p')], authors: [], thumbs: {}, loading: false });
	}
	// Переоткрыли самый старый до переполнения: он снова свежий.
	rememberSearchScreen(keys[0], { hits: [hit('p')], authors: [], thumbs: {}, loading: false });
	expect(recallSearchScreen(keys[0])).toBeDefined();
	expect(recallSearchScreen(keys[1])).toBeUndefined();
});

it('выход из входа очищает память поиска', () => {
	rememberSearchScreen(key, { hits: [hit('p1')], authors: [], thumbs: {}, loading: false });
	forgetSearchScreens();
	expect(recallSearchScreen(key)).toBeUndefined();
});
