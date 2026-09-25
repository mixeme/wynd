import { WORD, plural } from '$lib/format/plural';
import type { CircleSearchHit } from './types';

/**
 * Состояние фильтров поиска и строка «нашлось столько-то».
 *
 * Жило в экране поиска: «1 записи» там считались тернарниками по `< 5`, а у
 * дней было своё развёрнутое правило (RDB-1, склонения). Адресная часть
 * состояния уже лежит в `journal/search.ts` — здесь то, что осталось.
 */
export interface ChipState {
	author: string;
	periodActive: boolean;
	periodFrom: string;
	periodTo: string;
}

/** Нажали на автора: тот же — снять фильтр, другой — поставить. */
export function toggleAuthor(state: ChipState, name: string): ChipState {
	return { ...state, author: state.author === name ? '' : name };
}

/** Выключенный отрезок дат забывает свои концы, чтобы не всплыть при включении. */
export function togglePeriod(state: ChipState): ChipState {
	if (state.periodActive) {
		return { ...state, periodActive: false, periodFrom: '', periodTo: '' };
	}
	return { ...state, periodActive: true };
}

/** Строка под полем: «2 записи, 3 комментария и 1 день». */
export function searchStats(hits: CircleSearchHit[]): string {
	const parts: string[] = [];
	const count = (kind: string) => hits.filter((h) => h.kind === kind).length;
	const posts = count('post');
	const comments = count('comment');
	const days = count('day');
	if (posts) parts.push(plural(posts, WORD.post));
	if (comments) parts.push(plural(comments, WORD.comment));
	if (days) parts.push(plural(days, WORD.day));
	return parts.join(', ').replace(/, ([^,]+)$/, ' и $1');
}
