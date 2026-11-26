import { androidPort } from '$lib/android/bridge';
import { desktopBridge } from '$lib/desktop/bridge';
import { runningStandalone } from '$lib/pwa/install';

/**
 * Баннер «Wynd на главный экран» (план 74): в браузере, когда сайт ещё не
 * стоит как приложение. В установленной PWA и в окнах Wynd баннера нет —
 * человек уже в приложении. Chrome и Vivaldi ставят системным окном
 * (install.ts); Firefox и Safari такого окна не дают — там баннер
 * показывает шаги меню.
 */
const KEY = 'wynd:pwa-hint-closed';

/** Firefox на компьютере веб-приложения не ставит: шаги вели бы в пустоту. */
function firefoxDesktop(ua: string): boolean {
	return /Firefox\//.test(ua) && !/Android|Mobile/i.test(ua);
}

export function showPwaHint(
	ua = typeof navigator === 'undefined' ? '' : navigator.userAgent,
	standalone = runningStandalone()
): boolean {
	if (androidPort() || desktopBridge()) return false;
	if (standalone) return false;
	if (firefoxDesktop(ua)) return false;
	try {
		return localStorage.getItem(KEY) !== '1';
	} catch {
		// Хранилища нет — закрыть было бы нельзя, лучше не показывать.
		return false;
	}
}

export function closePwaHint(): void {
	try {
		localStorage.setItem(KEY, '1');
	} catch {
		/* не страшно */
	}
}

/**
 * Что нажать, чтобы поставить веб-приложение: шаги меню по браузеру.
 * Показывается, когда системного окна установки нет.
 */
export function pwaInstallSteps(
	ua = typeof navigator === 'undefined' ? '' : navigator.userAgent
): string {
	const ipadDesktop =
		/^Macintosh/.test(ua) &&
		typeof navigator !== 'undefined' &&
		(navigator.maxTouchPoints ?? 0) > 1;
	if (/iPhone|iPad|iPod/i.test(ua) || ipadDesktop)
		return 'В Safari коснитесь «Поделиться» и выберите «На экран „Домой“».';
	if (/Firefox\//.test(ua))
		return 'В меню Firefox выберите «Установить» или «Добавить на главный экран».';
	if (/Safari\//.test(ua) && !/Chrome|Chromium|Edg\//.test(ua))
		return 'В меню «Файл» выберите «Добавить в Dock».';
	if (/Android|Mobile/i.test(ua)) return 'В меню браузера выберите «Установить приложение».';
	return 'В адресной строке браузера нажмите значок установки.';
}
