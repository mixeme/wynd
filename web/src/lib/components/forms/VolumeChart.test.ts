import { flushSync, mount, unmount } from 'svelte';
import { describe, expect, it, vi } from 'vitest';
import VolumeChart from './VolumeChart.svelte';

const volume = Array.from({ length: 24 }, (_, i) => ({
	period: `${2024 + Math.floor(i / 12)}-${String((i % 12) + 1).padStart(2, '0')}`,
	bytes: (i + 1) * 1000,
	cumulative_bytes: ((i + 1) * (i + 2) * 1000) / 2
}));

function key(el: Element, k: string) {
	el.dispatchEvent(new KeyboardEvent('keydown', { key: k, bubbles: true }));
	flushSync();
}

// Инвариант (план 42, UI-4): ползунок отсечки достижим Tab и двигается
// клавишами: стрелки — месяц, PageUp/PageDown — год, Home/End — края.
describe('VolumeChart keyboard', () => {
	it('moves the cutoff with keys', () => {
		const oncutoff = vi.fn();
		const target = document.createElement('div');
		document.body.append(target);
		const instance = mount(VolumeChart, { target, props: { volume, oncutoff } });
		flushSync();
		const slider = target.querySelector('[role="slider"]');
		expect(slider?.getAttribute('tabindex')).toBe('0');

		key(slider!, 'Home');
		expect(oncutoff).toHaveBeenLastCalledWith(0);
		key(slider!, 'ArrowRight');
		expect(oncutoff).toHaveBeenLastCalledWith(1);
		key(slider!, 'PageUp');
		expect(oncutoff).toHaveBeenLastCalledWith(13);
		key(slider!, 'End');
		expect(oncutoff).toHaveBeenLastCalledWith(23);
		key(slider!, 'ArrowRight');
		expect(oncutoff).toHaveBeenLastCalledWith(23);

		unmount(instance);
		target.remove();
	});

	it('is not focusable without oncutoff', () => {
		const target = document.createElement('div');
		const instance = mount(VolumeChart, { target, props: { volume } });
		flushSync();
		expect(target.querySelector('svg')?.hasAttribute('tabindex')).toBe(false);
		unmount(instance);
	});
});
