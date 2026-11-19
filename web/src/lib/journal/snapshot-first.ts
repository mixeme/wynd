/**
 * Первый кадр вкладки круга из снимка (C100): экран рисует уже сохранённое на
 * устройстве, сеть уточняет следом. Тот же приём, что в ленте (C97), — здесь
 * он общий для «Сетки», «Дней», дня и «Карты», чтобы вкладки не расходились.
 *
 * Каждый загрузчик сам решает, что рисовать из снимка, и возвращает функцию
 * отрисовки либо `undefined`, если снимка нет. Свежий ответ не перетирается:
 * страница помечает его через `fresh`, и слишком поздно пришедший снимок
 * ничего не меняет.
 */
export type SnapshotPaint = () => void;

export async function paintFromSnapshots(
	fresh: () => boolean,
	...loads: Array<() => Promise<SnapshotPaint | undefined>>
): Promise<boolean> {
	const paints = (await Promise.all(loads.map((load) => load().catch(() => undefined)))).filter(
		(paint): paint is SnapshotPaint => Boolean(paint)
	);
	if (!paints.length || fresh()) return false;
	for (const paint of paints) paint();
	return true;
}
