// @ts-nocheck — Node-скрипт сторожа; типы не тянем в svelte-check.
// Сторож диалогов (C156): действия диалога не уходят под клавиатуру.
//
// Диалог рисуется поверх окна с фиксированным верхом; высоту видимой области
// знает только `--app-h` из session/viewport.ts. Без неё в удалении круга
// (6.40) «Отмена»/«Удалить» оказывались под клавиатурой, а диалог приходилось
// закрывать и открывать — владелец видел мигание. Правило `body .dlg` держит
// верх и высоту по `--app-h`.
import fs from 'node:fs';
import path from 'node:path';

const FILE = 'src/lib/styles/ui.css';

/**
 * Объявления всех правил `body .dlg { … }`: селектор стоит и в общем списке
 * фиксированных оверлеев, и отдельным правилом диалога. `.dlg-fit-viewport`
 * и прочие варианты не считаются — смотрим только сам диалог.
 */
function blocks(text) {
	const out = [];
	for (const m of text.matchAll(/(?:^|\n)body \.dlg \{/g)) {
		const start = text.indexOf('{', m.index);
		const end = text.indexOf('}', start);
		if (start < 0 || end < 0) continue;
		out.push(text.slice(start + 1, end));
	}
	return out;
}

/**
 * Что в правиле диалога перестало считаться с видимой областью.
 * @param {string} webRoot
 * @returns {string[]}
 */
export function checkDialogFit(webRoot) {
	let text;
	try {
		text = fs.readFileSync(path.join(webRoot, FILE), 'utf8');
	} catch {
		return [`${FILE}: файла стилей нет`];
	}
	const rules = blocks(text);
	if (!rules.length) return ['body .dlg: правила нет — диалог снова не вписан в видимую область'];
	const fits = (what) => rules.some((rule) => what.test(rule));
	const hits = [];
	if (!fits(/--dialog-top\s*:[^;]*--app-h/)) {
		hits.push('body .dlg: top не считается по --app-h (клавиатура прячет действия)');
	}
	if (!fits(/max-height\s*:[^;]*--app-h/)) {
		hits.push('body .dlg: max-height не считается по --app-h (короткое окно обрезает диалог)');
	}
	return hits;
}
