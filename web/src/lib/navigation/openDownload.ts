/**
 * Скачивание по ссылке сервера (свой архив, полный архив круга): качает сам
 * браузер, со своим ходом, без копии файла в памяти вкладки.
 *
 * На iPhone в приложении с экрана «Домой» ссылка открывалась в самом окне
 * приложения: просмотр ZIP или ответ сервера вставали на место Wynd, а в
 * этом окне нет ни адресной строки, ни «Назад» — выйти можно было, только
 * закрыв приложение (2026-10-23). Там ссылка уходит в новое окно: iOS
 * открывает его поверх приложения, с кнопкой «Готово».
 */
export function openDownload(href: string): void {
	const a = document.createElement('a');
	a.href = href;
	a.rel = 'noopener';
	if (isIosHomeScreenApp()) a.target = '_blank';
	a.click();
}

/** Reserve the iPhone window during the tap, before fetching a one-time link. */
export async function openDownloadAfter(loadHref: () => Promise<string>): Promise<void> {
	if (!isIosHomeScreenApp()) {
		openDownload(await loadHref());
		return;
	}
	// Opening _blank after a network request can lose user activation. Keep
	// the window reference without an opener; no token or session goes here.
	const download = window.open('about:blank', '_blank');
	if (!download) throw new Error('download_window_blocked');
	try {
		download.opener = null;
		const href = await loadHref();
		// Closing the window while the link is being fetched cancels the handoff.
		if (!download.closed) download.location.replace(href);
	} catch (err) {
		if (!download.closed) download.close();
		throw err;
	}
}

/** `navigator.standalone` есть только у Safari на iPhone и iPad. */
function isIosHomeScreenApp(): boolean {
	return (navigator as Navigator & { standalone?: boolean }).standalone === true;
}
