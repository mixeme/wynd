// Ширина окна для раскладки (план 75): на широком экране улочка и круг
// стоят двумя колонками. Порог тот же, что в CSS `.ph.app.split` (720 px):
// ниже — телефонная колонка, от 720 — список слева и круг справа.
// Сжатый вид списка и ширина колонок — только в CSS, отдельного состояния нет.
const QUERY = '(min-width: 720px)';

function media(): MediaQueryList | undefined {
	return typeof window === 'undefined' ? undefined : window.matchMedia(QUERY);
}

let split = $state(media()?.matches ?? false);

/** Две колонки, когда ширина позволяет. */
export function splitLayout(): boolean {
	return split;
}

/** Слушает смену ширины; зовётся один раз из корневого layout. */
export function initLayoutMode(): () => void {
	const mq = media();
	if (!mq) return () => {};
	split = mq.matches;
	// Читаем `mq.matches`, а не событие: так же ведёт себя подмена в тестах.
	const onChange = () => {
		split = mq.matches;
	};
	mq.addEventListener('change', onChange);
	return () => mq.removeEventListener('change', onChange);
}
