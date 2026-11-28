/**
 * Место для «Карты» (план 46, A4): один запрос при открытии экрана.
 *
 * Без ограничения по времени (C163): раньше запрос жил 8 секунд — меньше,
 * чем проходят объяснение «Место на карте» и системный запрос, и первый
 * вызов уходил в ошибку до выдачи разрешения; карта оставалась на точках
 * круга, а центрировалась только повторное открытие, когда разрешение уже
 * выдано и координаты приходят мгновенно. Ответ приходит, когда разрешение
 * решено: отказ и выключенное «Местоположение» дают ошибку сразу — остаётся
 * вид на все точки.
 */
export function requestDeviceHere(
	onHere: (here: [number, number]) => void,
	geolocation: Geolocation | undefined = typeof navigator === 'undefined'
		? undefined
		: navigator.geolocation
): void {
	if (!window.isSecureContext || !geolocation) return;
	geolocation.getCurrentPosition(
		(pos) => onHere([pos.coords.latitude, pos.coords.longitude]),
		() => {
			/* не дали или не нашли — остаётся вид на все точки */
		},
		{ timeout: Infinity, maximumAge: 10 * 60 * 1000, enableHighAccuracy: false }
	);
}
