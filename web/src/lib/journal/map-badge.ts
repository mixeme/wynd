/**
 * Счётчик «Карты» (C111): с местом считаются записи, а не точки. У записи с
 * двумя фото с координатами две точки, а запись одна.
 */
export function postsWithPlace(pins: Array<{ post_id: string }>): number {
	return new Set(pins.map((pin) => pin.post_id)).size;
}
