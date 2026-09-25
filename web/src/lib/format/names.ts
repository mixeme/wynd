/** Дательный падеж имени: «Аня» → «Ане» (заголовок передачи владения). */
export function toDativeName(name: string): string {
	const trimmed = name.trim();
	if (!trimmed) return name;
	const last = trimmed.slice(-1);
	const stem = trimmed.slice(0, -1);
	if (last === 'я' || last === 'а') return stem + 'е';
	if (last === 'й' || last === 'ь') return stem + (last === 'ь' ? 'и' : 'ю');
	if (/[бвгджзклмнпрстфхцчшщ]$/i.test(last)) return trimmed + 'у';
	return trimmed;
}

/** Винительный для названия круга в «Передать «…»»: «Семья» → «Семью». */
export function toAccusativeTitle(name: string): string {
	const trimmed = name.trim();
	if (!trimmed) return name;
	const last = trimmed.slice(-1);
	const stem = trimmed.slice(0, -1);
	if (last === 'я' || last === 'а') return stem + 'ю';
	return trimmed;
}
