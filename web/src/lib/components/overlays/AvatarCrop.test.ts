import { flushSync, mount, unmount } from 'svelte';
import { beforeAll, describe, expect, it } from 'vitest';
import AvatarCrop from './AvatarCrop.svelte';

// jsdom не даёт ResizeObserver: окно кадра следит за своим размером.
beforeAll(() => {
	globalThis.ResizeObserver ??= class {
		observe() {}
		unobserve() {}
		disconnect() {}
	} as unknown as typeof ResizeObserver;
});

function render(): HTMLElement {
	const target = document.createElement('div');
	const instance = mount(AvatarCrop, {
		target,
		props: { color: 'terracotta', oncancel: () => {} }
	});
	flushSync();
	const copy = target.cloneNode(true) as HTMLElement;
	unmount(instance);
	return copy;
}

describe('AvatarCrop', () => {
	// Кадр — не отдельная шапка: та же полоса редактора (4.3, 6.13), 48 px.
	it('берёт общую шапку редактора без нижнего запаса', () => {
		const bar = render().querySelector('.cbar');
		expect(bar?.querySelectorAll(':scope > div')).toHaveLength(1);
		expect(bar?.querySelector('.top.compose-top .t')?.textContent).toBe('Кадр');
	});
});
