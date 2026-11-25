import { mount, unmount } from 'svelte';
import { describe, expect, it, vi } from 'vitest';
import ReactionBar from './ReactionBar.svelte';

const KEYS = [
	{ key: 'heart', emoji: '❤️', label: 'Сердце' },
	{ key: 'laugh', emoji: '😄', label: 'Смех' },
	{ key: 'surprise', emoji: '😮', label: 'Удивление' },
	{ key: 'anger', emoji: '😠', label: 'Злость' }
];

describe('ReactionBar', () => {
	it('renders groups and calls onopenList', () => {
		const onopenList = vi.fn();
		const onadd = vi.fn();
		const target = document.createElement('div');
		const instance = mount(ReactionBar, {
			target,
			props: {
				groups: [{ emoji: '❤️', names: 'Аня' }],
				keys: KEYS,
				showAdd: true,
				onopenList,
				onadd,
				onpick: () => {}
			}
		});
		expect(target.textContent).toContain('❤️');
		expect(target.textContent).toContain('Аня');
		(target.querySelector('button.one') as HTMLButtonElement | null)?.click();
		expect(onopenList).toHaveBeenCalledOnce();
		(target.querySelector('button.add') as HTMLButtonElement | null)?.click();
		expect(onadd).toHaveBeenCalledOnce();
		unmount(instance);
	});

	it('shows picker and calls onpick with the server key', () => {
		const onpick = vi.fn();
		const target = document.createElement('div');
		const instance = mount(ReactionBar, {
			target,
			props: {
				groups: [],
				keys: KEYS,
				pickerOpen: true,
				onopenList: () => {},
				onadd: () => {},
				onpick
			}
		});
		expect(target.querySelectorAll('button.rcho')).toHaveLength(4);
		(target.querySelector('button.rcho') as HTMLButtonElement | null)?.click();
		expect(onpick).toHaveBeenCalledWith('heart');
		unmount(instance);
	});
});
