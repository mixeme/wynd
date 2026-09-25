import { isEditableActive } from '$lib/format/time';
import { ownReaction } from '$lib/journal/present';
import type { Reaction } from '$lib/journal/types';

/**
 * Реакция «моя» на экране: кто её ставит и как она выглядит до ответа
 * сервера. Одна и та же логика стояла в ленте и в обсуждении слово в слово
 * (RDB-1) — здесь она одна и под тестом.
 */
export interface ReactionActor {
	identityId: string;
	identityName: string;
}

/** Показывать ли «плюс»: реакция одна на человека и живёт в своём окне правок. */
export function canReact(
	reactions: Reaction[] | undefined,
	actor: ReactionActor,
	opts: { canWrite: boolean; solo: boolean; locked: boolean }
): boolean {
	if (!opts.canWrite || opts.solo || opts.locked) return false;
	const mine = ownReaction(reactions, actor.identityId);
	if (!mine) return true;
	return isEditableActive(mine.editable_until);
}

/**
 * Отражает свою реакцию в списке до ответа сервера (офлайн-очередь):
 * emoji = null снимает, иначе ставит или заменяет свою. Чужие не трогает.
 */
export function applyOwnReaction(
	reactions: Reaction[] | undefined,
	postId: string,
	emoji: string | null,
	actor: ReactionActor,
	now: Date = new Date()
): Reaction[] {
	const next = [...(reactions ?? [])];
	const idx = next.findIndex((r) => r.identity_id === actor.identityId);
	if (emoji === null) {
		if (idx >= 0) next.splice(idx, 1);
		return next;
	}
	if (idx >= 0) {
		next[idx] = { ...next[idx], emoji };
		return next;
	}
	next.push({
		id: `local-${postId}`,
		post_id: postId,
		emoji,
		author_name: actor.identityName,
		identity_id: actor.identityId,
		created_at: now.toISOString()
	});
	return next;
}
