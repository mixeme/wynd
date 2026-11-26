import { afterEach, describe, expect, it } from 'vitest';
import { clearWideSource, setSearchSource, wideSource } from './wideSource.svelte';

afterEach(() => {
	clearWideSource();
	sessionStorage.clear();
});

describe('wideSource', () => {
	it('запоминает адрес поиска и очищается', () => {
		expect(wideSource()).toBeNull();
		setSearchSource('/search?q=дача');
		expect(wideSource()).toEqual({ kind: 'search', url: '/search?q=дача' });
		clearWideSource();
		expect(wideSource()).toBeNull();
	});
});
