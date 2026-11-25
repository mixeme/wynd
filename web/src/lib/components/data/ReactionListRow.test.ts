import { mount, unmount } from 'svelte';
import { describe, expect, it } from 'vitest';
import ReactionListRow from './ReactionListRow.svelte';

describe('ReactionListRow', () => {
	it('renders div.row2 with name and emoji', () => {
		const target = document.createElement('div');
		const instance = mount(ReactionListRow, {
			target,
			props: { initial: 'К', name: 'Кот', color: '#62452F', emoji: '❤️' }
		});
		expect(target.querySelector('div.row2')?.tagName).toBe('DIV');
		expect(target.querySelector('button')).toBeNull();
		expect(target.textContent).toContain('Кот');
		expect(target.textContent).toContain('❤️');
		unmount(instance);
	});
});
