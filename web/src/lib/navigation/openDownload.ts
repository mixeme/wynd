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

/** `navigator.standalone` есть только у Safari на iPhone и iPad. */
function isIosHomeScreenApp(): boolean {
	return (navigator as Navigator & { standalone?: boolean }).standalone === true;
}
