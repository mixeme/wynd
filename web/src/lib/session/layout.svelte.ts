// Ширина окна для раскладки (план 75): на широком экране улочка и круг
// стоят двумя колонками. До 720 px — телефонная колонка; 720–999 px — рейл
// из аватаров и круг 720; от 1000 px — полный список 340 и круг 720.
// Пороги те же, что в CSS `.ph.app.split` / `.ph.app.split.rail`.

/** Раскладка окна: телефон, рейл аватаров, полный список с кругом. */
export type LayoutMode = 'phone' | 'rail' | 'full';

const SPLIT_QUERY = '(min-width: 720px)';
const FULL_QUERY = '(min-width: 1000px)';

function media(query: string): MediaQueryList | undefined {
	return typeof window === 'undefined' ? undefined : window.matchMedia(query);
}

function current(): LayoutMode {
	const split = media(SPLIT_QUERY)?.matches ?? false;
	const full = media(FULL_QUERY)?.matches ?? false;
	return !split ? 'phone' : full ? 'full' : 'rail';
}

let mode = $state<LayoutMode>(current());

/** Текущая раскладка окна. */
export function layoutMode(): LayoutMode {
	return mode;
}

/** Две колонки, когда ширина позволяет. */
export function splitLayout(): boolean {
	return mode !== 'phone';
}

/** Промежуточная ширина: от кругов остаются только аватары. */
export function railLayout(): boolean {
	return mode === 'rail';
}

/** Слушает смену ширины; зовётся один раз из корневого layout. */
export function initLayoutMode(): () => void {
	const splitMq = media(SPLIT_QUERY);
	const fullMq = media(FULL_QUERY);
	if (!splitMq || !fullMq) return () => {};
	const update = () => {
		mode = !splitMq.matches ? 'phone' : fullMq.matches ? 'full' : 'rail';
	};
	update();
	// Читаем `mq.matches`, а не событие: так же ведёт себя подмена в тестах.
	const onChange = () => update();
	splitMq.addEventListener('change', onChange);
	fullMq.addEventListener('change', onChange);
	return () => {
		splitMq.removeEventListener('change', onChange);
		fullMq.removeEventListener('change', onChange);
	};
}
