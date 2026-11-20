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

/**
 * Что делает касание вкладки (C106). На экране самой вкладки повторное
 * касание только поднимает список наверх; на чужом экране (день, альбом дня)
 * вкладка — обычный переход, даже если в полоске она уже подсвечена.
 */
export type TabTap = 'navigate' | 'scroll' | 'none';

export function tabTap(active: string, tapped: string, atTab: boolean, hasCircle: boolean): TabTap {
	if (!hasCircle) return 'none';
	if (tapped !== active) return 'navigate';
	return atTab ? 'scroll' : 'navigate';
}
