// «Место со снимков» — личная настройка участника в круге (wynd.html,
// «Геолокация»): уходят ли координаты из EXIF вместе с фото в этот круг.
// Хранится на сервере (memberships.share_place) и приходит в карточке круга —
// одна на все устройства. В 0.12.0 она жила в браузере и терялась на втором
// телефоне; настройка круга так себя вести не может.
//
// Запись может отступить от настройки значком места на экране записи (4.2).

import { apiJson } from '$lib/api/client';

type Meta = { geo_lat?: number | null; geo_lng?: number | null };

export async function saveSharePlace(origin: string, circleId: string, on: boolean): Promise<boolean> {
	const out = await apiJson<{ share_place: boolean }>(origin, `/circles/${circleId}/place`, {
		method: 'PUT',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ share_place: on })
	});
	return out.share_place;
}

/** Метаданные вложения без координат, если место для записи выключено. */
export function withPlace<T extends Meta>(meta: T, usePlace: boolean): T {
	if (usePlace) return meta;
	return { ...meta, geo_lat: undefined, geo_lng: undefined };
}

export function hasPlace(meta: Meta): boolean {
	return meta.geo_lat != null && meta.geo_lng != null;
}
