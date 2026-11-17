import { isAccessError } from '$lib/api/client';
import { mergePages, refetchOlder } from '$lib/journal/pages';
import { unreadDividerIndex } from '$lib/journal/present';
import { applyOwnReaction, type ReactionActor } from '$lib/journal/reactions';
import type { FeedEvent, FeedPost, FeedSnapshot } from '$lib/journal/types';

/**
 * Данные ленты круга: первая порция, догрузка старших, граница непрочитанного
 * и ошибка загрузки. Разметка остаётся в экране, поведение проверяется тестом
 * (план 60, точечный рефакторинг крупных веб-экранов).
 */
export interface FeedDataDeps {
	load: () => Promise<FeedSnapshot>;
	loadOlder: (before: string) => Promise<{ items: FeedPost[]; next?: string }>;
	lastRead: () => number;
	/** Медиа новых записей: экран качает обложки и аватары пачками. */
	resolveMedia?: (items: FeedPost[]) => void;
	/** Обложки открыток дня — в тот же словарь, что обложки записей. */
	resolveDayMedia?: (events: FeedEvent[]) => void;
}

export class FeedData {
	posts = $state<FeedPost[]>([]);
	// Догруженные старшие порции (C18): снимок — самые новые записи, остальное
	// приходит при прокрутке к концу ленты.
	olderPosts = $state<FeedPost[]>([]);
	nextBefore = $state<string | undefined>(undefined);
	loadingOlder = $state(false);
	feedEvents = $state<FeedEvent[]>([]);
	visibleFrom = $state<string | null>(null);
	circleStartedAt = $state('');
	dividerAt = $state<number | null>(null);
	loading = $state(true);
	error = $state('');

	private deps: FeedDataDeps;

	constructor(deps: FeedDataDeps) {
		this.deps = deps;
	}

	/** Первая порция; догруженные старшие перечитываются от новой границы. */
	async load(): Promise<void> {
		this.error = '';
		try {
			const snap = await this.deps.load();
			this.nextBefore = snap.next_before;
			if (this.olderPosts.length) {
				// Граница первой порции сдвинулась — перечитываем догруженное от неё.
				const again = await refetchOlder(
					this.deps.loadOlder,
					snap.next_before,
					this.olderPosts.length
				).catch(() => ({ items: this.olderPosts, next: this.nextBefore }));
				this.olderPosts = again.items;
				this.nextBefore = again.next;
			}
			this.posts = mergePages(snap.posts, this.olderPosts, (p) => p.id);
			this.feedEvents = snap.events ?? [];
			this.visibleFrom = snap.visible_from ?? null;
			this.circleStartedAt = snap.circle_started_at ?? '';
			this.dividerAt = unreadDividerIndex(this.posts, this.deps.lastRead());
			this.deps.resolveMedia?.(this.posts);
			this.deps.resolveDayMedia?.(this.feedEvents);
		} catch (err) {
			this.error = isAccessError(err) ? 'Нет доступа' : 'Не удалось загрузить ленту';
		} finally {
			this.loading = false;
		}
	}

	/** Старшая порция по прокрутке; без курсора или во время загрузки — ничего. */
	async loadOlder(): Promise<void> {
		if (!this.nextBefore || this.loadingOlder) return;
		this.loadingOlder = true;
		try {
			const page = await this.deps.loadOlder(this.nextBefore);
			this.olderPosts = [...this.olderPosts, ...page.items];
			this.nextBefore = page.next;
			this.posts = mergePages(this.posts, page.items, (p) => p.id);
			this.deps.resolveMedia?.(page.items);
		} catch {
			// Без сети старшее не догрузится — лента остаётся как есть.
		} finally {
			this.loadingOlder = false;
		}
	}

	/** Своя реакция на экране до ответа сервера (офлайн-очередь). */
	applyReaction(postId: string, emoji: string | null, actor: ReactionActor): void {
		this.posts = this.posts.map((p) =>
			p.id === postId ? { ...p, reactions: applyOwnReaction(p.reactions, postId, emoji, actor) } : p
		);
	}

	setError(text: string): void {
		this.error = text;
	}

	clearError(): void {
		this.error = '';
	}
}
