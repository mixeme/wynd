import type { IconName } from '$ui/Icon.svelte';
import type { Comment, FeedEvent, FeedPost, MediaSummary, Reaction } from './types';

export { REACTION_KEYS } from './types';
export type { ReactionKey } from './types';

export function authorInitial(name: string): string {
	const t = name.trim();
	return t ? t[0].toUpperCase() : '?';
}

export function reactionNames(reactions: Reaction[] | undefined): string {
	if (!reactions?.length) return '';
	return reactions.map((r) => r.author_name).join(', ');
}

export function reactionIconName(emoji: string): IconName {
	if (emoji === 'laugh' || emoji === 'surprise' || emoji === 'anger') return emoji;
	return 'heart';
}

export function groupReactions(
	reactions: Reaction[] | undefined
): { emoji: string; names: string }[] {
	if (!reactions?.length) return [];
	const order: string[] = [];
	const byEmoji = new Map<string, string[]>();
	for (const reaction of reactions) {
		if (!byEmoji.has(reaction.emoji)) {
			byEmoji.set(reaction.emoji, []);
			order.push(reaction.emoji);
		}
		byEmoji.get(reaction.emoji)!.push(reaction.author_name);
	}
	return order.map((emoji) => ({
		emoji,
		names: byEmoji.get(emoji)!.join(', ')
	}));
}

export function ownReaction(reactions: Reaction[] | undefined, identityId: string): Reaction | undefined {
	return reactions?.find((r) => r.identity_id === identityId);
}

export function commentPreview(comments: Comment[] | undefined): {
	first?: string;
	more: number;
} {
	if (!comments?.length) return { more: 0 };
	const first = comments[0];
	const line = `${first.author_name}: ${first.body}`;
	return { first: line, more: Math.max(0, comments.length - 1) };
}

export function coverMedia(media: MediaSummary[] | undefined): MediaSummary | undefined {
	const visual = photoMedia(media);
	if (!visual.length) return undefined;
	return visual.find((m) => m.is_cover) ?? visual.find((m) => m.kind === 'photo') ?? visual[0];
}

export function photoMedia(media: MediaSummary[] | undefined): MediaSummary[] {
	return media?.filter((m) => m.kind === 'photo' || m.kind === 'video') ?? [];
}

export function attachmentMedia(media: MediaSummary[] | undefined): MediaSummary[] {
	return media?.filter((m) => m.kind === 'attachment') ?? [];
}

export function attachmentLabel(att: MediaSummary): string {
	return att.filename?.trim() || 'Вложение';
}

export function attachmentSizeLabel(att: MediaSummary, formatBytes: (n: number) => string): string {
	if (att.size_bytes != null && att.size_bytes > 0) return formatBytes(att.size_bytes);
	return 'скачать';
}

export function mediaCount(media: MediaSummary[] | undefined): number {
	return photoMedia(media).length;
}

export function locationLabel(media: MediaSummary | undefined): string | undefined {
	if (!media?.geo_lat || !media?.geo_lng) return undefined;
	return 'На карте';
}

export function findPost(posts: FeedPost[], postId: string): FeedPost | undefined {
	return posts.find((p) => p.id === postId);
}

/** Индекс черты непрочитанного: перед первым прочитанным постом. */
export function unreadDividerIndex(posts: FeedPost[], lastReadSeq: number): number | null {
	if (!posts.length || lastReadSeq <= 0) return null;
	for (let i = 0; i < posts.length; i++) {
		if (posts[i].event_seq <= lastReadSeq) {
			if (i > 0 && posts[i - 1].event_seq > lastReadSeq) return i;
			return null;
		}
	}
	if (posts[0].event_seq > lastReadSeq) return posts.length;
	return null;
}

export function maxReadSeq(posts: FeedPost[]): number {
	let max = 0;
	for (const p of posts) {
		if (p.event_seq > max) max = p.event_seq;
	}
	return max;
}

/** Service events strictly between two post event_seq values (feed order: upper is newer). */
export function serviceEventsBetween(
	events: FeedEvent[],
	upperSeq: number,
	lowerSeq: number
): FeedEvent[] {
	return events
		.filter((e) => e.seq > lowerSeq && e.seq < upperSeq)
		.sort((a, b) => b.seq - a.seq);
}
