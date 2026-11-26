/**
 * Выбор из левого списка на широком экране (план 76).
 *
 * Первый раздел открывается новой записью истории: позади остаётся список,
 * куда «Назад» и возвращает. Смена уже открытого раздела заменяет запись,
 * иначе «Назад» мотал бы открытые круги, настройки и результаты поиска.
 */
export function detailEntryOptions(hasOpenDetail: boolean): { replaceState: true } | undefined {
	return hasOpenDetail ? { replaceState: true } : undefined;
}
