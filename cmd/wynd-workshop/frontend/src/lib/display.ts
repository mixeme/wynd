// Формат отображения пульта и осмотров: размеры, даты, пометки, места.
// Чистые функции — тот же текст, что был в App.svelte до выноса (план 60).
import type { Finding, ServerPage } from '../api';

export function gb(bytes: number): string {
	if (bytes >= 1024 ** 3) return (bytes / 1024 ** 3).toFixed(1).replace('.', ',') + ' ГБ';
	return Math.max(1, Math.round(bytes / 1024 ** 2)) + ' МБ';
}

export function day(iso: string): string {
	const d = new Date(iso);
	return Number.isNaN(d.getTime())
		? iso
		: d.toLocaleDateString('ru-RU', { day: 'numeric', month: 'long' });
}

export function dayTime(iso: string): string {
	const d = new Date(iso);
	if (Number.isNaN(d.getTime())) return iso;
	return `${day(iso)} в ${d.toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit' })}`;
}

export function mark(level: Finding['level']): string {
	return level === 'ok' ? '✓' : '!';
}

// Кадр 7б2: места не хватает даже на копию базы.
export function noRoom(p: ServerPage | undefined): boolean {
	return Boolean(
		p && p.state.free_bytes > 0 && p.state.free_bytes < p.copy_bytes + 64 * 1024 * 1024
	);
}
