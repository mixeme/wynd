// Ширина окна для раскладки (план 76): на широком экране у раздела своя
// левая колонка, а справа — выбранное. До 1060 px — телефонная колонка; от
// 1060 px — две колонки: список 340 и содержимое 720, ровно столько, сколько
// занимает полный каркас. Порог тот же, что в CSS `.ph.app.split`.

/** Раскладка окна: телефон и две колонки. */
export type LayoutMode = 'phone' | 'full';

const FULL_QUERY = '(min-width: 1060px)';

function media(query: string): MediaQueryList | undefined {
	return typeof window === 'undefined' ? undefined : window.matchMedia(query);
}

function current(): LayoutMode {
	return media(FULL_QUERY)?.matches ? 'full' : 'phone';
}

let mode = $state<LayoutMode>(current());

/** Текущая раскладка окна. */
export function layoutMode(): LayoutMode {
	return mode;
}

/** Две колонки, когда ширина позволяет. */
export function splitLayout(): boolean {
	return mode === 'full';
}

/** Слушает смену ширины; зовётся один раз из корневого layout. */
export function initLayoutMode(): () => void {
	const fullMq = media(FULL_QUERY);
	if (!fullMq) return () => {};
	const update = () => {
		mode = fullMq.matches ? 'full' : 'phone';
	};
	update();
	// Читаем `mq.matches`, а не событие: так же ведёт себя подмена в тестах.
	const onChange = () => update();
	fullMq.addEventListener('change', onChange);
	return () => {
		fullMq.removeEventListener('change', onChange);
	};
}
