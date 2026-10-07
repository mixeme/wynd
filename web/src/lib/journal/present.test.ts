import { describe, expect, it } from 'vitest';
import {
	commentMediaItems,
	commentPhotoIds,
	dayCoverTileId,
	isLocalPosterId,
	localPosterId,
	tileBlobId,
	attachmentBlocks,
	isAttachedToOtherDay,
	localDayOf,
	albumCompressionHint,
	albumDownloadFilename,
	audioRowLabel,
	audioTimeLabel,
	commentPreview,
	formatAudioClock,
	groupReactions,
	isAudioMedia,
	lightboxCaption,
	reactionIconName,
	serviceEventsAboveNewest,
	serviceEventsBetween,
	unreadDividerIndex
} from './present';
import type { Comment, FeedPost, Reaction } from './types';

const at = '2026-01-01T00:00:00Z';

describe('tile of a video', () => {
	it('draws a photo as itself', () => {
		expect(tileBlobId({ blob_id: 'p', kind: 'photo' })).toBe('p');
	});

	it('draws a video by its poster, never by the file', () => {
		expect(tileBlobId({ blob_id: 'v', kind: 'video', video_poster_blob_id: 'j' })).toBe('j');
		const local = tileBlobId({ blob_id: 'v', kind: 'video' });
		expect(local).toBe(localPosterId('v'));
		expect(isLocalPosterId(local)).toBe(true);
		expect(isLocalPosterId('v')).toBe(false);
	});

	it('picks the day cover picture the same way', () => {
		const day = { entry_date: '2026-10-04', post_count: 1 };
		expect(dayCoverTileId({ ...day, fallback_cover_blob_id: 'p' })).toBe('p');
		expect(
			dayCoverTileId({ ...day, cover_blob_id: 'v', cover_is_video: true, cover_image_blob_id: 'j' })
		).toBe('j');
		expect(dayCoverTileId({ ...day, fallback_cover_blob_id: 'v', cover_is_video: true })).toBe(
			localPosterId('v')
		);
		expect(dayCoverTileId({ ...day })).toBeUndefined();
		// Открытка дня: своя обложка есть или нет, запасной не бывает.
		const card = { entry_date: '2026-08-06', caption: 'Кот выбрал обложку' };
		expect(dayCoverTileId({ ...card, cover_blob_id: 'p' })).toBe('p');
		expect(dayCoverTileId(card)).toBeUndefined();
	});
});

describe('present', () => {
	it('groups reactions by emoji preserving first-seen order', () => {
		const reactions: Reaction[] = [
			{ id: 'r1', post_id: 'p', emoji: 'heart', author_name: 'Аня', identity_id: 'a', created_at: at },
			{ id: 'r2', post_id: 'p', emoji: 'laugh', author_name: 'Боб', identity_id: 'b', created_at: at },
			{ id: 'r3', post_id: 'p', emoji: 'heart', author_name: 'Вера', identity_id: 'c', created_at: at }
		];
		expect(groupReactions(reactions)).toEqual([
			{ emoji: 'heart', names: 'Аня, Вера' },
			{ emoji: 'laugh', names: 'Боб' }
		]);
	});

	it('maps reaction emoji to icon name', () => {
		expect(reactionIconName('laugh')).toBe('laugh');
		expect(reactionIconName('surprise')).toBe('surprise');
		expect(reactionIconName('anger')).toBe('anger');
		expect(reactionIconName('heart')).toBe('heart');
		expect(reactionIconName('unknown')).toBe('heart');
	});

	it('builds comment preview with more count', () => {
		const comments: Comment[] = [
			{ id: 'c1', post_id: 'p', author_name: 'Аня', body: 'привет', identity_id: 'a', created_at: at },
			{ id: 'c2', post_id: 'p', author_name: 'Боб', body: 'ответ', identity_id: 'b', created_at: '2026-01-01T00:01:00Z' },
			{ id: 'c3', post_id: 'p', author_name: 'Вера', body: 'ещё', identity_id: 'c', created_at: '2026-01-01T00:02:00Z' }
		];
		expect(commentPreview(comments)).toEqual({
			first: 'Вера: ещё',
			createdAt: '2026-01-01T00:02:00Z',
			more: 2,
			comment: comments[2]
		});
		expect(commentPreview([])).toEqual({ more: 0 });
		// Реплика без слов (4.30): имя, а вложение превью покажет само.
		const voice: Comment = {
			id: 'c4', post_id: 'p', author_name: 'Кот', body: '', identity_id: 'k', created_at: at,
			media: [{ blob_id: 'v', kind: 'attachment', is_cover: false, voice: true, audio_duration_ms: 48000 }]
		};
		expect(commentPreview([voice]).first).toBe('Кот:');
	});

	it('describes comment media for display', () => {
		const items = commentMediaItems(
			[
				{ blob_id: 'p1', kind: 'photo', is_cover: false },
				{ blob_id: 'v', kind: 'attachment', is_cover: false, voice: true, audio_duration_ms: 48000, audio_peaks: [5] },
				{ blob_id: 'f', kind: 'attachment', is_cover: false, filename: 'list.pdf', size_bytes: 240 }
			],
			{ p1: 'blob:1' }
		);
		expect(items).toEqual([
			{ key: 'p1', kind: 'photo', blobId: 'p1', url: 'blob:1' },
			{ key: 'v', kind: 'voice', blobId: 'v', peaks: [5], durationMs: 48000 },
			{ key: 'f', kind: 'file', blobId: 'f', name: 'list.pdf', size: 240 }
		]);
		expect(commentMediaItems(undefined, {})).toEqual([]);
		expect(commentPhotoIds([{ blob_id: 'p1', kind: 'photo', is_cover: false }, { blob_id: 'f', kind: 'attachment', is_cover: false }])).toEqual(['p1']);
	});

	it('places unread divider before first read post', () => {
		const posts: FeedPost[] = [
			{ id: 'p3', event_seq: 30, body: '', entry_date: '2026-01-01', author_name: '', identity_id: '', created_at: at },
			{ id: 'p2', event_seq: 20, body: '', entry_date: '2026-01-01', author_name: '', identity_id: '', created_at: at },
			{ id: 'p1', event_seq: 10, body: '', entry_date: '2026-01-01', author_name: '', identity_id: '', created_at: at }
		];
		expect(unreadDividerIndex(posts, 15)).toBe(2);
		expect(unreadDividerIndex(posts, 0)).toBeNull();
		expect(unreadDividerIndex(posts, 30)).toBeNull();
		expect(
			unreadDividerIndex(
				[{ id: 'p', event_seq: 5, body: '', entry_date: '2026-01-01', author_name: '', identity_id: '', created_at: at }],
				0
			)
		).toBeNull();
		expect(
			unreadDividerIndex(
				[{ id: 'p', event_seq: 5, body: '', entry_date: '2026-01-01', author_name: '', identity_id: '', created_at: at }],
				1
			)
		).toBe(1);
	});

	it('names lightbox downloads from filename or kind and 1-based index', () => {
		expect(
			albumDownloadFilename(
				{ blob_id: 'a', kind: 'photo', is_cover: false, filename: ' IMG.JPG ' },
				0
			)
		).toBe('IMG.JPG');
		expect(albumDownloadFilename({ blob_id: 'a', kind: 'photo', is_cover: false }, 0)).toBe(
			'photo-1.jpg'
		);
		expect(albumDownloadFilename({ blob_id: 'b', kind: 'video', is_cover: false }, 2)).toBe(
			'video-3.mp4'
		);
	});

	it('shows service events above the newest post and when feed is empty', () => {
		const events = [
			{ seq: 5, summary: 'Мышка теперь Мышь', created_at: at },
			{ seq: 3, summary: 'Боб вступил', created_at: at }
		];
		expect(serviceEventsAboveNewest(events, 4).map((e) => e.seq)).toEqual([5]);
		expect(serviceEventsAboveNewest(events, 5).map((e) => e.seq)).toEqual([]);
		expect(serviceEventsAboveNewest(events, undefined).map((e) => e.seq)).toEqual([5, 3]);
		expect(serviceEventsBetween(events, 10, 0).map((e) => e.seq)).toEqual([5, 3]);
	});

	it('treats audio mime and sound extensions as playable, and a pdf as a file', () => {
		expect(isAudioMedia('audio/mpeg', 'song.bin')).toBe(true);
		expect(isAudioMedia('', 'track.m4a')).toBe(true);
		expect(isAudioMedia('application/octet-stream', 'voice.ogg')).toBe(true);
		expect(isAudioMedia('application/pdf', 'notes.mp3')).toBe(false);
		expect(isAudioMedia('image/jpeg', 'cover.jpg')).toBe(false);
	});

	it('uses artist and title together, otherwise the filename', () => {
		const base = { blob_id: 'a', kind: 'attachment' as const, is_cover: false, filename: 'a.mp3' };
		expect(audioRowLabel({ ...base, audio_artist: 'Бригада', audio_title: 'Утро' })).toBe(
			'Бригада — Утро'
		);
		expect(audioRowLabel({ ...base, audio_artist: 'Бригада' })).toBe('a.mp3');
		expect(audioRowLabel({ ...base, audio_title: 'Утро' })).toBe('a.mp3');
		expect(audioRowLabel(base)).toBe('a.mp3');
	});

	it('formats the play clock and stays empty until playback starts', () => {
		expect(formatAudioClock(0)).toBe('0:00');
		expect(formatAudioClock(42)).toBe('0:42');
		expect(formatAudioClock(111)).toBe('1:51');
		expect(formatAudioClock(3661)).toBe('1:01:01');
		expect(audioTimeLabel(false, 10, 111)).toBe('0:00');
		expect(audioTimeLabel(true, 42, 111)).toBe('0:42 · 1:51');
		expect(audioTimeLabel(true, 5, 0)).toBe('0:05');
	});

	it('puts the photo size in the lightbox caption, not under the album', () => {
		const post = {
			id: 'p',
			author_name: 'Аня',
			created_at: '2026-08-12T11:02:00Z',
			captured_at: '2026-08-12T10:00:00Z',
			media: []
		} as unknown as FeedPost;
		const fmt = () => 'сегодня, 14:02';
		expect(lightboxCaption(post, undefined, fmt, { width: 2048, height: 1536 })).toBe(
			'Аня · сегодня, 14:02 · 2048 × 1536'
		);
		expect(lightboxCaption(post, undefined, fmt)).toBe('Аня · сегодня, 14:02');
		expect(albumCompressionHint(post)).toBe('снято 12 августа · файлы сжаты');
		expect(albumCompressionHint(post)).not.toMatch(/px/);
	});

	it('звуки подряд — одна группа, файл между ними её разрывает', () => {
		const a = (id: string, mime: string, filename: string) =>
			({ blob_id: id, kind: 'attachment', is_cover: false, mime_type: mime, filename }) as const;
		const blocks = attachmentBlocks([
			a('1', 'audio/mpeg', 'a.mp3'),
			a('2', '', 'b.m4a'),
			a('3', 'application/pdf', 'c.pdf'),
			a('4', 'audio/ogg', 'd.ogg')
		]);
		expect(blocks.map((b) => (b.kind === 'audio' ? b.items.map((i) => i.blob_id).join('+') : b.item.blob_id))).toEqual([
			'1+2',
			'3',
			'4'
		]);
	});

	it('день привязки показывается, только когда отличается от дня публикации', () => {
		const at = new Date(2026, 8, 30, 23, 14).toISOString();
		expect(localDayOf(at)).toBe('2026-09-30');
		expect(isAttachedToOtherDay({ entry_date: '2026-09-30', created_at: at })).toBe(false);
		expect(isAttachedToOtherDay({ entry_date: '2026-09-27', created_at: at })).toBe(true);
	});
});
