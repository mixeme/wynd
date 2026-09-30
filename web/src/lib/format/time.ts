import { WORD, plural } from './plural';

const timeFmt = new Intl.DateTimeFormat('ru-RU', { hour: '2-digit', minute: '2-digit' });
const dateFmt = new Intl.DateTimeFormat('ru-RU', { day: 'numeric', month: 'long' });
const dayYearFmt = new Intl.DateTimeFormat('ru-RU', {
	day: 'numeric',
	month: 'long',
	year: 'numeric'
});

function startOfDay(d: Date): Date {
	return new Date(d.getFullYear(), d.getMonth(), d.getDate());
}

function daysBetween(a: Date, b: Date): number {
	const ms = startOfDay(a).getTime() - startOfDay(b).getTime();
	return Math.round(ms / 86_400_000);
}

/** Строка-разделитель дня в «Откликах» (3.13): «сегодня», «вчера», «6 августа». */
export function formatDayLabel(iso: string, now: Date = new Date()): string {
	const at = new Date(iso);
	const diff = daysBetween(now, at);
	if (diff === 0) return 'сегодня';
	if (diff === 1) return 'вчера';
	return dateFmt.format(at);
}

/** Локальный календарный день метки — ключ для разделителей. */
export function localDayKey(iso: string): string {
	const d = new Date(iso);
	return `${d.getFullYear()}-${d.getMonth() + 1}-${d.getDate()}`;
}

/** Только часы для реплики в обсуждении: «14:20». */
export function formatClock(createdAt: string): string {
	return timeFmt.format(new Date(createdAt));
}

export function formatPostTime(createdAt: string, entryDate?: string): string {
	const created = new Date(createdAt);
	const now = new Date();
	const diff = daysBetween(now, created);
	const time = timeFmt.format(created);

	if (diff === 0) return `сегодня, ${time}`;
	if (diff === 1) return `вчера, ${time}`;

	if (entryDate) {
		const [y, m, d] = entryDate.split('-').map(Number);
		const entry = new Date(y, m - 1, d);
		if (daysBetween(entry, created) !== 0) {
			return `${dateFmt.format(entry)}, ${time}`;
		}
	}

	return `${dateFmt.format(created)}, ${time}`;
}

/** Короткая дата для бейджа «задним числом». */
export function formatEntryDate(entryDate: string): string {
	if (!entryDate) return '';
	const [y, m, d] = entryDate.split('-').map(Number);
	return dateFmt.format(new Date(y, m - 1, d));
}

/** День входа на устройстве: локальный календарный день ISO-метки. */
export function formatSessionDay(iso: string): string {
	if (!iso) return '';
	const d = new Date(iso);
	if (Number.isNaN(d.getTime())) return '';
	return dateFmt.format(d);
}

/**
 * Дата в панели: «15 сентября» и «15 сентября 2026». Три копии с одним и
 * тем же `Intl.DateTimeFormat` жили в экранах хранилища, людей и учётки
 * (план 42, волна 5). Нечитаемую метку отдаём как есть — в панели лучше
 * показать сырое значение, чем пустоту.
 */
export function formatAdminDay(iso: string): string {
	const d = new Date(iso);
	if (Number.isNaN(d.getTime())) return iso;
	return dateFmt.format(d);
}

export function formatAdminDayYear(iso: string): string {
	const d = new Date(iso);
	if (Number.isNaN(d.getTime())) return iso;
	return dayYearFmt.format(d);
}

/** Дедлайн архивации: «до 15 сентября». */
export function formatDeadline(iso: string): string {
	if (!iso) return '';
	const d = new Date(iso);
	if (Number.isNaN(d.getTime())) return '';
	return `до ${dateFmt.format(d)}`;
}

/** Дней до даты (ISO), не меньше нуля. */
export function daysUntil(iso: string): number {
	const target = new Date(iso);
	const now = new Date();
	return Math.max(0, daysBetween(target, now));
}

/** Окно правок: «завтра, 14:02». */
export function formatEditableUntil(iso: string): string {
	const d = new Date(iso);
	const now = new Date();
	const diff = daysBetween(d, now);
	const time = timeFmt.format(d);
	if (diff === 0) return `сегодня, ${time}`;
	if (diff === 1) return `завтра, ${time}`;
	return `${dateFmt.format(d)}, ${time}`;
}

export function isEditableActive(editableUntil?: string | null): boolean {
	if (!editableUntil) return false;
	if (editableUntil.startsWith('9999-')) return true;
	return new Date(editableUntil) > new Date();
}

const monthYearFmt = new Intl.DateTimeFormat('ru-RU', { month: 'long', year: 'numeric' });

/** Месяц и год для сетки и дней: «Август 2026». */
export function formatMonthYear(entryDate: string): string {
	const [y, m] = entryDate.split('-').map(Number);
	return monthYearFmt.format(new Date(y, m - 1, 1));
}

/** Склонение «файл / файла / файлов». */
export function pluralFiles(count: number): string {
	return plural(count, WORD.file);
}

/** Склонение «фотография / фотографии / фотографий». */
export function pluralPhotos(count: number): string {
	return plural(count, WORD.photo);
}

/** Склонение «запись / записи / записей». */
export function pluralPosts(count: number): string {
	return plural(count, WORD.post);
}

/** Склонение «N человек / человека». */
export function pluralPeople(count: number): string {
	return plural(count, WORD.person);
}

/** Подпись карточки дня: «12 августа · 4 записи». */
export function formatDayCardSubtitle(entryDate: string, postCount: number): string {
	return `${formatEntryDate(entryDate)} · ${pluralPosts(postCount)}`;
}
