/**
 * Русское склонение по числу.
 *
 * Правило одно, а копий его было семь: пять почти одинаковых функций в
 * `format/time.ts` и `pay/pay.ts` и ещё четыре развёрнутых `mod10/mod100`
 * прямо в экранах. Мимо них жили строки вроде «ещё 1 комментариев».
 */
export type PluralForms = [one: string, few: string, many: string];

/** Форма слова для числа: 1 запись, 2 записи, 5 записей. */
export function pluralForm(count: number, forms: PluralForms): string {
	const n = Math.abs(Math.trunc(count));
	const mod10 = n % 10;
	const mod100 = n % 100;
	if (mod10 === 1 && mod100 !== 11) return forms[0];
	if (mod10 >= 2 && mod10 <= 4 && (mod100 < 10 || mod100 >= 20)) return forms[1];
	return forms[2];
}

/** Число и слово в нужной форме: «5 записей». */
export function plural(count: number, forms: PluralForms): string {
	return `${count} ${pluralForm(count, forms)}`;
}

export const WORD = {
	post: ['запись', 'записи', 'записей'] as PluralForms,
	comment: ['комментарий', 'комментария', 'комментариев'] as PluralForms,
	day: ['день', 'дня', 'дней'] as PluralForms,
	file: ['файл', 'файла', 'файлов'] as PluralForms,
	photo: ['фотография', 'фотографии', 'фотографий'] as PluralForms,
	person: ['человек', 'человека', 'человек'] as PluralForms,
	circle: ['круг', 'круга', 'кругов'] as PluralForms,
	yourCircle: ['ваш круг', 'ваших круга', 'ваших кругов'] as PluralForms
} as const;
