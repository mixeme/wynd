// @ts-nocheck — Node-скрипт сторожа; типы не тянем в svelte-check.
// Сторож первого кадра возврата (C160): экран поиска помнит свой кадр, поле
// не фокусируется на шаге по истории.
//
// При возврате с записи экран поиска монтируется заново. Пока строки жили
// только в компоненте, на кадр появлялись «Ищем…» и пустое место, а
// `SearchField` повторно фокусировал поле — в PWA Firefox это поднимало
// клавиатуру (запись владельца «И снова экран мигает»). Механизм состоит из
// трёх частей: память кадра в экране, гейт фокуса по типу перехода и сам
// `SearchField`, который спрашивает гейт. Общий случай («у любого экрана с
// сетевыми данными есть источник первого кадра») статически не проверяется —
// это правило ревью в docs/testing/ui-review-protocol.md.
import fs from 'node:fs';
import path from 'node:path';

const SEARCH_PAGE = 'src/routes/circles/[id]/search/+page.svelte';
const SEARCH_SCOPE = 'src/lib/journal/search-scope.ts';
const SEARCH_FIELD = 'src/lib/components/forms/SearchField.svelte';

function read(webRoot, rel) {
	try {
		return fs.readFileSync(path.join(webRoot, rel), 'utf8');
	} catch {
		return undefined;
	}
}

/**
 * Что из механизма первого кадра возврата пропало.
 * @param {string} webRoot
 * @returns {string[]}
 */
export function checkFirstFrameReturn(webRoot) {
	const hits = [];
	const page = read(webRoot, SEARCH_PAGE);
	if (page === undefined) {
		hits.push(`${SEARCH_PAGE}: файла нет`);
	} else {
		const recall = page.indexOf('recallSearchScreen(');
		const hitsInit = page.indexOf('let hits = $state');
		if (recall < 0) {
			hits.push(`${SEARCH_PAGE}: нет recallSearchScreen — возврат с записи снова покажет «Ищем…»`);
		}
		if (!page.includes('rememberSearchScreen(')) {
			hits.push(`${SEARCH_PAGE}: кадр не запоминается (rememberSearchScreen) — память пуста`);
		}
		if (recall >= 0 && hitsInit >= 0 && recall > hitsInit) {
			hits.push(`${SEARCH_PAGE}: кадр вспоминается после инициализации строк — первый кадр пуст`);
		}
	}
	const scope = read(webRoot, SEARCH_SCOPE);
	if (scope === undefined) {
		hits.push(`${SEARCH_SCOPE}: файла нет`);
	} else {
		if (!scope.includes('export function noteArrival(')) {
			hits.push(`${SEARCH_SCOPE}: нет noteArrival — тип перехода не доходит до фокуса`);
		}
		if (!scope.includes("'popstate'")) {
			hits.push(`${SEARCH_SCOPE}: поиск не различает шаг по истории (popstate)`);
		}
	}
	const field = read(webRoot, SEARCH_FIELD);
	if (field === undefined) {
		hits.push(`${SEARCH_FIELD}: файла нет`);
	} else if (!field.includes('searchFocusAfterNavigation(')) {
		hits.push(
			`${SEARCH_FIELD}: фокус без searchFocusAfterNavigation (гейта нет) — клавиатура снова выедет на возврате`
		);
	}
	return hits;
}
