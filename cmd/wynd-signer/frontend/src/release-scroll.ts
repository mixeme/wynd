// Якорь прокрутки экрана выпуска (12в/12г): где читают и куда вернуть после
// подмены списка этапов. Вынесено из App.svelte ради проверки: снимок позиции,
// взятый один раз при нажатии, возвращал окно к началу на каждом этапе.
export interface ReleaseScroll {
	top: number;
	/** Человек читал у конца — после подмены ведём за новыми этапами. */
	atEnd: boolean;
}

export function readReleaseScroll(
	scrollTop: number,
	scrollHeight: number,
	clientHeight: number
): ReleaseScroll {
	const max = Math.max(0, scrollHeight - clientHeight);
	return {
		top: Math.max(0, scrollTop),
		// Экран, который целиком помещается, не «читают у конца»: после роста
		// содержимого остаёмся у начала, а не прыгаем вниз.
		atEnd: max > 0 && max - scrollTop <= 2
	};
}

export function releaseScrollTarget(
	position: ReleaseScroll,
	scrollHeight: number,
	clientHeight: number,
	done: boolean
): number {
	if (done) return Math.max(0, scrollHeight);
	const max = Math.max(0, scrollHeight - clientHeight);
	return position.atEnd ? max : Math.min(position.top, max);
}
