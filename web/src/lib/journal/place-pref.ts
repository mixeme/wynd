// «Место со снимков» — личная настройка круга (wynd.html, «Геолокация»):
// уходят ли координаты из EXIF вместе с фото в этот круг. Живёт на
// устройстве, а не на сервере: координаты берутся из снимков этого же
// устройства, и серверу не нужно знать даже сам выбор. По умолчанию
// включено — так приложение вело себя до настройки.
//
// Запись может отступить от настройки значком места на экране записи (4.2).

type Meta = { geo_lat?: number | null; geo_lng?: number | null };

function key(origin: string, circleId: string): string {
	return `wynd.place.${origin || 'self'}.${circleId}`;
}

export function getPlacePref(origin: string, circleId: string): boolean {
	try {
		return localStorage.getItem(key(origin, circleId)) !== 'off';
	} catch {
		return true;
	}
}

export function setPlacePref(origin: string, circleId: string, on: boolean): void {
	try {
		if (on) localStorage.removeItem(key(origin, circleId));
		else localStorage.setItem(key(origin, circleId), 'off');
	} catch {
		/* приватный режим — настройка живёт до конца вкладки */
	}
}

/** Метаданные вложения без координат, если место для записи выключено. */
export function withPlace<T extends Meta>(meta: T, usePlace: boolean): T {
	if (usePlace) return meta;
	return { ...meta, geo_lat: undefined, geo_lng: undefined };
}

export function hasPlace(meta: Meta): boolean {
	return meta.geo_lat != null && meta.geo_lng != null;
}
