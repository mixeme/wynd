import type { IconName } from '$ui/Icon.svelte';
import type { Comment, DaySummary, FeedEvent, FeedPost, MediaSummary, Reaction } from './types';

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
	createdAt?: string;
	more: number;
} {
	if (!comments?.length) return { more: 0 };
	// Список с сервера идёт от старого к новому. В ленте — последняя реплика.
	let latest = comments[0];
	for (const comment of comments) {
		if (comment.created_at >= latest.created_at) latest = comment;
	}
	const line = `${latest.author_name}: ${latest.body}`;
	return { first: line, createdAt: latest.created_at, more: Math.max(0, comments.length - 1) };
}

export function coverMedia(media: MediaSummary[] | undefined): MediaSummary | undefined {
	const visual = photoMedia(media);
	if (!visual.length) return undefined;
	return visual.find((m) => m.is_cover) ?? visual.find((m) => m.kind === 'photo') ?? visual[0];
}

/**
 * Касание обложки: несколько снимков — альбом записи, один — сразу он во весь
 * экран (лайтбокс альбома), листать там нечего.
 */
export function albumHref(circleId: string, postId: string, visualCount: number): string {
	const base = `/circles/${circleId}/posts/${postId}/album`;
	return visualCount === 1 ? `${base}?lb=0&single=1` : base;
}

const LOCAL_POSTER_PREFIX = 'local-poster:';

/**
 * Ключ кадра, снятого на этом устройстве: ролик отправили без кадра, а здесь
 * его уже открывали. На сервере такого блоба нет — только кэш устройства.
 */
export function localPosterId(videoBlobId: string): string {
	return LOCAL_POSTER_PREFIX + videoBlobId;
}

export function isLocalPosterId(blobId: string): boolean {
	return blobId.startsWith(LOCAL_POSTER_PREFIX);
}

/**
 * Что рисует плитка: фото — само себя, ролик — JPEG своего кадра. Сам ролик
 * ради плитки не качают: лента с десятком роликов вешала телефон (план 49).
 */
export function tileBlobId(
	media: Pick<MediaSummary, 'blob_id' | 'kind' | 'video_poster_blob_id'>
): string {
	if (media.kind !== 'video') return media.blob_id;
	return media.video_poster_blob_id || localPosterId(media.blob_id);
}

/** Картинка обложки дня: у ролика — его кадр, как в плитке. */
export function dayCoverTileId(day: DaySummary | undefined): string | undefined {
	const fileId = day?.cover_blob_id ?? day?.fallback_cover_blob_id;
	if (!fileId || !day?.cover_is_video) return fileId;
	return day.cover_image_blob_id || localPosterId(fileId);
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

const AUDIO_EXT = new Set(['m4a', 'mp3', 'aac', 'ogg', 'opus', 'wav', 'flac']);

/** Звук — audio/*, а при пустом типе или octet-stream ещё и по расширению. */
export function isAudioMedia(mime?: string, filename?: string): boolean {
	const raw = (mime ?? '').toLowerCase();
	const semi = raw.indexOf(';');
	const m = (semi >= 0 ? raw.slice(0, semi) : raw).trim();
	if (m.startsWith('audio/')) return true;
	if (m && m !== 'application/octet-stream') return false;
	const ext = filename?.split('.').pop()?.toLowerCase() ?? '';
	return AUDIO_EXT.has(ext);
}

/**
 * Вложения записи по порядку, но звуки подряд — одной группой: несколько
 * звуков рисуются одной рамкой (4.18), одиночный — как на 4.15.
 */
export type AttachmentBlock =
	| { kind: 'audio'; items: MediaSummary[] }
	| { kind: 'file'; item: MediaSummary };

export function attachmentBlocks(list: MediaSummary[]): AttachmentBlock[] {
	const out: AttachmentBlock[] = [];
	for (const att of list) {
		if (isAudioMedia(att.mime_type, att.filename)) {
			const last = out[out.length - 1];
			if (last?.kind === 'audio') last.items.push(att);
			else out.push({ kind: 'audio', items: [att] });
		} else {
			out.push({ kind: 'file', item: att });
		}
	}
	return out;
}

/** Местный календарный день метки: «2026-09-30». */
export function localDayOf(iso: string): string {
	const d = new Date(iso);
	const mm = String(d.getMonth() + 1).padStart(2, '0');
	const dd = String(d.getDate()).padStart(2, '0');
	return `${d.getFullYear()}-${mm}-${dd}`;
}

/**
 * Запись привязана к другому дню, чем опубликована (3.4): в ленте справа
 * в шапке — день привязки. Сравнивается с днём публикации, а не с сегодня:
 * иначе пометку получала любая вчерашняя запись.
 */
export function isAttachedToOtherDay(post: Pick<FeedPost, 'entry_date' | 'created_at'>): boolean {
	return Boolean(post.entry_date) && post.entry_date !== localDayOf(post.created_at);
}

/** Оба тега — «исполнитель — название». Одного или пустых нет: остаётся имя файла. */
export function audioRowLabel(att: MediaSummary): string {
	const artist = att.audio_artist?.trim() ?? '';
	const title = att.audio_title?.trim() ?? '';
	if (artist && title) return `${artist} — ${title}`;
	return attachmentLabel(att);
}

export function formatAudioClock(seconds: number): string {
	if (!Number.isFinite(seconds) || seconds < 0) seconds = 0;
	const whole = Math.floor(seconds);
	const ss = String(whole % 60).padStart(2, '0');
	const mins = Math.floor(whole / 60);
	if (mins >= 60) {
		const h = Math.floor(mins / 60);
		return `${h}:${String(mins % 60).padStart(2, '0')}:${ss}`;
	}
	return `${mins}:${ss}`;
}

/** До первого запуска — «0:00». После — «0:42 · 1:51», а без длительности только прошедшее. */
export function audioTimeLabel(started: boolean, current: number, duration: number): string {
	if (!started) return '0:00';
	const now = formatAudioClock(current);
	if (!Number.isFinite(duration) || duration <= 0) return now;
	return `${now} · ${formatAudioClock(duration)}`;
}

/** Имя файла при скачивании из лайтбокса альбома (4.14). */
export function albumDownloadFilename(media: MediaSummary, index: number): string {
	const name = media.filename?.trim();
	if (name) return name;
	const n = index + 1;
	return media.kind === 'video' ? `video-${n}.mp4` : `photo-${n}.jpg`;
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

const shotDateFmt = new Intl.DateTimeFormat('ru-RU', { day: 'numeric', month: 'long' });

/** Строка под сеткой альбома (4.13): когда снято и что файлы сжаты. Размер
 *  каждого снимка — в его подписи на 4.14, а не одной цифрой настройки. */
export function albumCompressionHint(post: FeedPost): string {
	const captured =
		post.captured_at ?? post.media?.find((m) => m.captured_at)?.captured_at;
	const parts: string[] = [];
	if (captured) parts.push(`снято ${shotDateFmt.format(new Date(captured))}`);
	parts.push(captured ? 'файлы сжаты' : 'файлы сжаты, оригиналы на телефоне');
	return parts.join(' · ');
}

export interface MediaSize {
	width: number;
	height: number;
}

/** Подпись лайтбокса: автор · время · место · размер снимка (макет 4.14). */
export function lightboxCaption(
	post: FeedPost,
	media: MediaSummary | undefined,
	formatPostTime: (createdAt: string, entryDate?: string) => string,
	size?: MediaSize
): string {
	const parts = [post.author_name, formatPostTime(post.created_at, post.entry_date)];
	const loc = locationLabel(media);
	if (loc) parts.push(loc);
	if (size && size.width > 0 && size.height > 0) parts.push(`${size.width} × ${size.height}`);
	return parts.join(' · ');
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

/** Service events newer than the top post (shown above the first card). */
export function serviceEventsAboveNewest(events: FeedEvent[], newestPostSeq: number | undefined): FeedEvent[] {
	if (!events.length) return [];
	if (newestPostSeq == null || newestPostSeq <= 0) {
		return [...events].sort((a, b) => b.seq - a.seq);
	}
	return events.filter((e) => e.seq > newestPostSeq).sort((a, b) => b.seq - a.seq);
}
