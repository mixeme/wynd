/**
 * Установка веб-приложения на главный экран (план 74). Chromium присылает
 * `beforeinstallprompt`, когда сайт можно поставить; событие перехватываем и
 * зовём из своего баннера. Firefox и Safari такого события не дают — там
 * баннер показывает шаги меню (pwaHint.ts).
 */
export interface InstallPromptEvent extends Event {
	prompt(): Promise<void>;
	userChoice: Promise<{ outcome: 'accepted' | 'dismissed' }>;
}

let pending: InstallPromptEvent | undefined;

/** Ловит событие установки. Возвращает отписку. */
export function captureInstallPrompt(): () => void {
	if (typeof window === 'undefined') return () => {};
	const onPrompt = (event: Event) => {
		event.preventDefault();
		pending = event as InstallPromptEvent;
	};
	window.addEventListener('beforeinstallprompt', onPrompt);
	return () => window.removeEventListener('beforeinstallprompt', onPrompt);
}

export type InstallOutcome = 'accepted' | 'dismissed' | 'unavailable';

/** Системное окно установки. Нет события — «unavailable», показываем шаги. */
export async function promptPwaInstall(): Promise<InstallOutcome> {
	const event = pending;
	if (!event) return 'unavailable';
	pending = undefined;
	try {
		await event.prompt();
		const choice = await event.userChoice;
		return choice.outcome;
	} catch {
		return 'unavailable';
	}
}

/** Сайт уже открыт как приложение с домашнего экрана. */
export function runningStandalone(): boolean {
	if (typeof window === 'undefined') return false;
	if (window.matchMedia?.('(display-mode: standalone)').matches) return true;
	const nav = navigator as Navigator & { standalone?: boolean };
	return nav.standalone === true;
}
