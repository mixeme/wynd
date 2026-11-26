import { describe, expect, it, vi } from 'vitest';
import { initLayoutMode, splitLayout } from './layout.svelte';

/** matchMedia, у которого ширину окна можно переключить на ходу. */
function installWidth(split: boolean) {
	const mq = new EventTarget() as EventTarget & { matches: boolean; media: string };
	mq.matches = split;
	mq.media = '(min-width: 720px)';
	const fn = vi.fn(() => mq);
	vi.stubGlobal('matchMedia', fn);
	Object.defineProperty(window, 'matchMedia', { writable: true, configurable: true, value: fn });
	return {
		set(next: boolean) {
			mq.matches = next;
			mq.dispatchEvent(new Event('change'));
		}
	};
}

// План 75: от 720 px раскладка двухколоночная, ниже — телефонная. Смена
// ширины видна сразу, после остановки слушателя — уже нет.
describe('layout mode', () => {
	it('follows the window width and stops listening', () => {
		const width = installWidth(false);
		const stop = initLayoutMode();

		expect(splitLayout()).toBe(false);
		width.set(true);
		expect(splitLayout()).toBe(true);
		width.set(false);
		expect(splitLayout()).toBe(false);

		stop();
		width.set(true);
		expect(splitLayout()).toBe(false);
	});
});
