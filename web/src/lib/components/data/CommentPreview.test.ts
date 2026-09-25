import { createRawSnippet, mount, unmount } from 'svelte';
import { describe, expect, it, vi } from 'vitest';
import CommentPreview from './CommentPreview.svelte';

const body = createRawSnippet(() => ({
	render: () => '<div class="mo">2 комментария</div>',
	setup: () => () => {}
}));

describe('CommentPreview', () => {
	it('renders button.cm and handles click', () => {
		const onclick = vi.fn();
		const target = document.createElement('div');
		const instance = mount(CommentPreview, {
			target,
			props: { onclick, children: body }
		});
		const btn = target.querySelector('button.cm');
		expect(btn?.tagName).toBe('BUTTON');
		(btn as HTMLButtonElement | null)?.click();
		expect(onclick).toHaveBeenCalledOnce();
		unmount(instance);
	});
});
