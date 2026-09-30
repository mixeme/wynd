import { describe, expect, it } from 'vitest';
import {
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
			more: 2
		});
		expect(commentPreview([])).toEqual({ more: 0 });
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
});
