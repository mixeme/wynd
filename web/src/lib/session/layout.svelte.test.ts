import { describe, expect, it, vi } from 'vitest';
import { initLayoutMode, layoutMode, splitLayout } from './layout.svelte';

const FULL_QUERY = '(min-width: 1060px)';

/** matchMedia с одним порогом 1060. */
function installWidth(full: boolean) {
	const mq = new EventTarget() as EventTarget & { matches: boolean; media: string };
	mq.matches = full;
	mq.media = FULL_QUERY;
	const fn = () => mq;
	vi.stubGlobal('matchMedia', fn);
	Object.defineProperty(window, 'matchMedia', { writable: true, configurable: true, value: fn });
	return {
		set(next: boolean) {
			mq.matches = next;
			mq.dispatchEvent(new Event('change'));
		}
	};
}

// План 76: до 1060 — одна колонка, от 1060 — две. Смена ширины видна сразу,
// после остановки слушателя — нет.
describe('layout mode', () => {
	it('follows the window width and stops listening', () => {
		const width = installWidth(false);
		const stop = initLayoutMode();

		expect(layoutMode()).toBe('phone');
		expect(splitLayout()).toBe(false);

		width.set(true);
		expect(layoutMode()).toBe('full');
		expect(splitLayout()).toBe(true);

		stop();
		width.set(false);
		expect(layoutMode()).toBe('full');
	});
});
