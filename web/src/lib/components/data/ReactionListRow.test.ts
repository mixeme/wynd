import { mount, unmount } from 'svelte';
import { describe, expect, it } from 'vitest';
import ReactionListRow from './ReactionListRow.svelte';

describe('ReactionListRow', () => {
	it('renders div.row2 with name and icon', () => {
		const target = document.createElement('div');
		const instance = mount(ReactionListRow, {
			target,
			props: { initial: 'К', name: 'Кот', color: '#62452F', icon: 'heart' }
		});
		expect(target.querySelector('div.row2')?.tagName).toBe('DIV');
		expect(target.querySelector('button')).toBeNull();
		expect(target.textContent).toContain('Кот');
		unmount(instance);
	});
});
