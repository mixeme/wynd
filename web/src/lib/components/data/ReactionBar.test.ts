import { mount, unmount } from 'svelte';
import { describe, expect, it, vi } from 'vitest';
import ReactionBar from './ReactionBar.svelte';

describe('ReactionBar', () => {
	it('renders groups and calls onopenList', () => {
		const onopenList = vi.fn();
		const onadd = vi.fn();
		const target = document.createElement('div');
		const instance = mount(ReactionBar, {
			target,
			props: {
				groups: [{ icon: 'heart', names: 'Аня' }],
				keys: ['heart', 'laugh', 'surprise', 'anger'],
				showAdd: true,
				onopenList,
				onadd,
				onpick: () => {}
			}
		});
		(target.querySelector('button.one') as HTMLButtonElement | null)?.click();
		expect(onopenList).toHaveBeenCalledOnce();
		(target.querySelector('button.add') as HTMLButtonElement | null)?.click();
		expect(onadd).toHaveBeenCalledOnce();
		unmount(instance);
	});

	it('shows picker and calls onpick', () => {
		const onpick = vi.fn();
		const target = document.createElement('div');
		const instance = mount(ReactionBar, {
			target,
			props: {
				groups: [],
				keys: ['heart', 'laugh', 'surprise', 'anger'],
				pickerOpen: true,
				onopenList: () => {},
				onadd: () => {},
				onpick
			}
		});
		(target.querySelector('button.rcho') as HTMLButtonElement | null)?.click();
		expect(onpick).toHaveBeenCalledWith('heart');
		unmount(instance);
	});
});
