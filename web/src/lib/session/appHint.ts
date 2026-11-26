import { androidPort } from '$lib/android/bridge';

/**
 * Баннер про приложение Wynd для Android (C139): на улочке в любом веб-
 * клиенте на Android — в браузере и в установленной PWA. В собственном окне
 * Wynd баннер не нужен: человек уже в приложении.
 * Закрыли — на этом устройстве больше не показываем (это про устройство,
 * а не про учётку), как у подсказки про браузер (7.7а).
 */
const KEY = 'wynd:android-app-hint-closed';

export function showAndroidAppHint(
	ua = typeof navigator === 'undefined' ? '' : navigator.userAgent
): boolean {
	if (androidPort()) return false;
	if (!/Android/i.test(ua)) return false;
	try {
		return localStorage.getItem(KEY) !== '1';
	} catch {
		// Хранилища нет — закрыть было бы нельзя, лучше не показывать.
		return false;
	}
}

export function closeAndroidAppHint(): void {
	try {
		localStorage.setItem(KEY, '1');
	} catch {
		/* не страшно */
	}
}
