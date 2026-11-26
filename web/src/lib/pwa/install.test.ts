import { afterEach, describe, expect, it, vi } from 'vitest';
import {
	captureInstallPrompt,
	promptPwaInstall,
	runningStandalone,
	type InstallPromptEvent
} from './install';

function installable(outcome: 'accepted' | 'dismissed'): InstallPromptEvent {
	const event = new Event('beforeinstallprompt') as InstallPromptEvent;
	event.prompt = vi.fn(async () => {});
	event.userChoice = Promise.resolve({ outcome });
	return event;
}

afterEach(() => {
	// События в очереди модуля не остаются: каждое забирается один раз.
	delete (navigator as unknown as { standalone?: boolean }).standalone;
});

// План 74: своё окно установки Chromium и определение «уже установлено».
describe('установка веб-приложения', () => {
	it('без события установки отвечает «unavailable»', async () => {
		expect(await promptPwaInstall()).toBe('unavailable');
	});

	it('зовёт системное окно и отдаёт выбор человека', async () => {
		const stop = captureInstallPrompt();
		const event = installable('accepted');
		window.dispatchEvent(event);
		expect(await promptPwaInstall()).toBe('accepted');
		// Событие одноразовое: следующий зов — снова шаги меню.
		expect(await promptPwaInstall()).toBe('unavailable');
		stop();
	});

	it('после отписки событие не перехватывается', async () => {
		const stop = captureInstallPrompt();
		stop();
		window.dispatchEvent(installable('dismissed'));
		expect(await promptPwaInstall()).toBe('unavailable');
	});

	it('узнаёт открытие из установленной PWA', () => {
		const original = window.matchMedia;
		window.matchMedia = ((query: string) => ({
			matches: query.includes('standalone'),
			media: query,
			onchange: null,
			addEventListener() {},
			removeEventListener() {},
			addListener() {},
			removeListener() {},
			dispatchEvent: () => false
		})) as typeof window.matchMedia;
		expect(runningStandalone()).toBe(true);
		window.matchMedia = original;
	});
});
