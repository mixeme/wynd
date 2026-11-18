import { goto } from '$app/navigation';

/**
 * Переключение вкладки круга — вид того же экрана, а не шаг истории.
 *
 * Запись истории заменяем: иначе «Назад» идёт обратно по переключениям
 * вкладок (Хронология → Сетка → Карта), а не на экран, откуда открыли круг
 * (владелец, 18.11). Источник и прокрутка остаются в записи под вкладкой,
 * поэтому возврат туда прежний.
 */
export function openCircleTab(circleId: string, suffix: string): Promise<void> {
	return goto(suffix ? `/circles/${circleId}${suffix}` : `/circles/${circleId}`, {
		replaceState: true
	});
}
