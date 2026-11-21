import { WORD, plural } from '$lib/format/plural';

/**
 * Строки сводок над списками круга — формы «из M» в родительном падеже
 * (C114: «из 2 записей», а не «из 2 записи»; у «Карты» то же с C111).
 */

/** «N дней из M записей» над списком «Дней». */
export function daysSummary(days: number, posts: number): string {
	return `${plural(days, WORD.day)} из ${plural(posts, WORD.postGenitive)}`;
}

/** «N записей с фото и видео из M записей» над плитками «Сетки». */
export function gridSummary(tiles: number, posts: number): string {
	return `${plural(tiles, WORD.post)} с фото и видео из ${plural(posts, WORD.postGenitive)}`;
}
