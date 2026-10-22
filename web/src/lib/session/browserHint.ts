/**
 * Подсказка про браузер над списком кругов (7.7а): на Android советуем
 * Chrome или Vivaldi. В Firefox установленный сайт закрывается при запуске
 * после простоя (проверено на squoosh.app — не Wynd) и видео не сжимается;
 * на Mi Browser были жалобы. Владелец, 2026-10-22: показывать в любом
 * браузере, кроме рекомендованных. Закрыли — не показываем на этом
 * устройстве: это про браузер, а не настройка учётки.
 */
const KEY = 'wynd:browser-hint-closed';

// Браузеры на движке Chrome, которые сами себя называют в строке браузера:
// это не Chrome, хотя «Chrome/» в строке у них есть.
const OTHER_CHROMIUM = /MiuiBrowser|XiaoMi|SamsungBrowser|OPR\/|EdgA\/|YaBrowser|UCBrowser|HuaweiBrowser|HeyTapBrowser|; wv\)/i;

/** Chrome или Vivaldi на Android. Vivaldi на Android называет себя как Chrome. */
export function isRecommendedAndroidBrowser(ua: string): boolean {
	if (/Vivaldi\//.test(ua)) return true;
	return /Chrome\/\d/.test(ua) && !OTHER_CHROMIUM.test(ua);
}

export function showBrowserHint(ua = typeof navigator === 'undefined' ? '' : navigator.userAgent): boolean {
	if (!/Android/i.test(ua) || isRecommendedAndroidBrowser(ua)) return false;
	try {
		return localStorage.getItem(KEY) !== '1';
	} catch {
		// Хранилища нет — закрыть было бы нельзя, лучше не показывать.
		return false;
	}
}

export function closeBrowserHint(): void {
	try {
		localStorage.setItem(KEY, '1');
	} catch {
		/* не страшно */
	}
}
