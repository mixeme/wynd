const units = ['Б', 'КБ', 'МБ', 'ГБ'] as const;

/** Размер файла: «1,2 МБ». */
export function formatBytes(bytes: number): string {
	if (bytes <= 0) return '0 Б';
	const i = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1);
	const value = bytes / 1024 ** i;
	const formatted =
		value >= 10 || i === 0 ? Math.round(value).toString() : value.toFixed(1).replace('.', ',');
	return `${formatted} ${units[i]}`;
}
