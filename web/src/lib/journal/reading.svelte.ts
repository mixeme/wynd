import { resolveMediaUrls } from '$lib/media/batch';
import { FeedData, type FeedDataDeps } from './feed-data.svelte';
import { loadFeed } from './feed';
import {
	attachmentMedia,
	commentPhotoIds,
	commentPreview,
	coverMedia,
	dayCoverTileId,
	tileBlobId
} from './present';
import { fetchOlderPage } from './pages';
import type { FeedEvent, FeedPost } from './types';

/**
 * Лента круга на время чтения (C95-1): экран ленты и экран записи делят один
 * контроллер и одни словари медиа. Запись открывается из загруженного без
 * сети, возврат в ленту сразу восстанавливает карточки и обложки, а
 * перечитывание идёт фоном. Запись живёт, пока открыт круг: при уходе со
 * круга или смене круга в макете её снимает dropCircleReading.
 */
export class CircleReading {
	readonly origin: string;
	readonly circleId: string;
	feed: FeedData;
	// Обложки и открытки дня: адрес blob: по id блоба.
	mediaUrls = $state<Record<string, string>>({});
	// Ход скачивания обложек и неудачи — плитка сама говорит, что происходит.
	mediaProgress = $state<Record<string, { received: number; total: number }>>({});
	mediaFailed = $state<Record<string, boolean>>({});
	// Лица авторов записей: адрес по id лица.
	authorAvatarUrls = $state<Record<string, string>>({});

	constructor(origin: string, circleId: string, lastRead: () => number) {
		this.origin = origin;
		this.circleId = circleId;
		this.feed = new FeedData(this.deps(lastRead));
	}

	/** Экран пересоздан: перенацеливаем зависимости на живой экран. */
	rebind(lastRead: () => number): void {
		this.feed.rebind(this.deps(lastRead));
	}

	/** Курсор прочитанного известен позже запроса ленты (C95-2). */
	redrawDivider(): void {
		this.feed.redrawDivider();
	}

	private deps(lastRead: () => number): FeedDataDeps {
		return {
			load: () => loadFeed(this.origin, this.circleId),
			loadOlder: async (before) => {
				const page = await fetchOlderPage<{ posts: FeedPost[] }>(
					this.origin,
					`/circles/${this.circleId}/feed`,
					before
				);
				return { items: page.posts, next: page.next_before };
			},
			lastRead,
			resolveMedia: (items) => void this.resolveMedia(items),
			resolveDayMedia: (events) => void this.resolveDayMedia(events)
		};
	}

	// Лента не ждёт картинок: пачки — общий хелпер $lib/media/batch (REF-6).
	// Лица и обложки — общие словари чтения: запись открывается с готовыми
	// адресами, возврат в ленту не перезагружает картинки.
	private async resolveMedia(feedPosts: FeedPost[]): Promise<void> {
		const avatarJobs = new Map<string, string>();
		const coverJobs = new Map<string, string>();
		for (const post of feedPosts) {
			const avatarBlob = post.author_avatar_blob_id;
			if (avatarBlob && !this.authorAvatarUrls[post.identity_id]) {
				avatarJobs.set(post.identity_id, avatarBlob);
			}
			const cover = coverMedia(post.media);
			if (cover && !this.mediaUrls[tileBlobId(cover)]) {
				coverJobs.set(tileBlobId(cover), tileBlobId(cover));
			}
			for (const att of attachmentMedia(post.media)) {
				if (att.audio_cover_blob_id && !this.mediaUrls[att.audio_cover_blob_id]) {
					coverJobs.set(att.audio_cover_blob_id, att.audio_cover_blob_id);
				}
			}
			// Снимки последней реплики — превью показывает их как в нити (4.22).
			for (const id of commentPhotoIds(commentPreview(post.comments).comment?.media).slice(0, 3)) {
				if (!this.mediaUrls[id]) coverJobs.set(id, id);
			}
		}

		// Аватар лица и обложка записи — разные ключи при одном блобе, поэтому
		// два прохода: сперва лица, потом обложки.
		const identitiesByBlob = new Map<string, string[]>();
		for (const [identityId, blobId] of avatarJobs) {
			// Один блоб может быть аватаром нескольких лиц — тогда адрес
			// достаётся всем им, а не последнему в списке.
			identitiesByBlob.set(blobId, [...(identitiesByBlob.get(blobId) ?? []), identityId]);
		}
		await resolveMediaUrls(this.origin, [...identitiesByBlob.keys()], (blobId, url) => {
			for (const identityId of identitiesByBlob.get(blobId) ?? []) {
				this.authorAvatarUrls = { ...this.authorAvatarUrls, [identityId]: url };
			}
		});
		await resolveMediaUrls(
			this.origin,
			[...coverJobs.keys()],
			(blobId, url) => {
				this.mediaUrls = { ...this.mediaUrls, [blobId]: url };
			},
			undefined,
			{
				onProgress: (blobId, received, total) => {
					this.mediaProgress = { ...this.mediaProgress, [blobId]: { received, total } };
				},
				onFail: (blobId) => {
					this.mediaFailed = { ...this.mediaFailed, [blobId]: true };
				}
			}
		);
	}

	// Обложки открыток дня (3.20) — в тот же словарь, что обложки записей.
	private async resolveDayMedia(events: FeedEvent[]): Promise<void> {
		const ids = new Set<string>();
		for (const ev of events) {
			const id = ev.day ? dayCoverTileId(ev.day) : undefined;
			if (id && !this.mediaUrls[id]) ids.add(id);
		}
		await resolveMediaUrls(this.origin, [...ids], (blobId, url) => {
			this.mediaUrls = { ...this.mediaUrls, [blobId]: url };
		});
	}
}

const reading = new Map<string, CircleReading>();

export function circleReading(
	origin: string,
	circleId: string,
	lastRead: () => number
): CircleReading {
	const key = `${origin}|${circleId}`;
	let entry = reading.get(key);
	if (!entry) {
		entry = new CircleReading(origin, circleId, lastRead);
		reading.set(key, entry);
	} else {
		entry.rebind(lastRead);
	}
	return entry;
}

/** Круг закрыт: загруженная страница больше не нужна. */
export function dropCircleReading(origin: string, circleId: string): void {
	reading.delete(`${origin}|${circleId}`);
}
