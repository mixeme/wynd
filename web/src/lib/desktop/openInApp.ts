// Кнопка «Открыть в приложении» на странице приглашения (план 66, функция 4;
// Linux — план 69): схему wynd: понимают окно Windows и приложение Linux.
// На телефоне обработчика схемы нет, а в окне мост уже есть и человек в
// приложении — там кнопка не нужна.
//
// Android в User-Agent тоже содержит «Linux», поэтому его проверяем первым;
// ChromeOS — «Linux» с «CrOS», для него обработчика нет.
export function canOpenInApp(userAgent: string | undefined): boolean {
	if (!userAgent) return false;
	if (/Android/i.test(userAgent)) return false;
	if (/CrOS/i.test(userAgent)) return false;
	if (/Windows/i.test(userAgent)) return true;
	return /Linux/i.test(userAgent);
}
