import { flushSync } from 'svelte';
import { describe, expect, it, vi } from 'vitest';
import * as session from './session.svelte';

/** matchMedia, у которого системную тему можно переключить на ходу. */
function installSystemTheme(dark: boolean) {
	const mq = new EventTarget() as EventTarget & { matches: boolean; media: string };
	mq.matches = dark;
	mq.media = '(prefers-color-scheme: dark)';
	const fn = vi.fn(() => mq);
	vi.stubGlobal('matchMedia', fn);
	Object.defineProperty(window, 'matchMedia', { writable: true, configurable: true, value: fn });
	return {
		mq,
		set(next: boolean) {
			mq.matches = next;
			mq.dispatchEvent(new Event('change'));
		}
	};
}

// Инвариант: тема «как в системе» следует за сменой системной темы без
// перезагрузки — класс body.dark и meta theme-color в корневом layout читают
// isDark() в эффекте и разметке, и те перерисовываются. Явная тема системную
// перекрывает; после остановки слушателя смена системы ничего не трогает.
// Модуль импортируется статически: после vi.resetModules у него была бы своя
// копия рантайма Svelte, и эффект теста не видел бы его состояние.
describe('system theme', () => {
	it('follows a live system theme change', () => {
		const system = installSystemTheme(false);
		const stop = session.initTheme();

		const seen: boolean[] = [];
		const cleanup = $effect.root(() => {
			$effect(() => {
				seen.push(session.isDark());
			});
		});
		flushSync();
		expect(seen).toEqual([false]);

		system.set(true);
		flushSync();
		expect(session.isDark()).toBe(true);
		expect(seen).toEqual([false, true]);

		session.setTheme('light');
		flushSync();
		expect(session.isDark()).toBe(false);
		system.set(false);
		system.set(true);
		flushSync();
		expect(session.isDark()).toBe(false);

		session.setTheme('system');
		flushSync();
		expect(session.isDark()).toBe(true);

		stop();
		system.set(false);
		flushSync();
		expect(session.isDark()).toBe(true);
		cleanup();
	});
});
