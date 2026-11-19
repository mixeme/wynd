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
	/** Снимок с устройства: первый кадр без сети, сеть — фоном (C97). */
	loadCached?: () => Promise<FeedSnapshot | undefined>;
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
	private inFlight: Promise<void> | undefined;
	private cachedFlight: Promise<void> | undefined;
	private cachedApplied = false;
	private freshApplied = false;

	constructor(deps: FeedDataDeps) {
		this.deps = deps;
	}

	/** Экран пересоздан (возврат в ленту): зависимости уходят на живой экран. */
	rebind(deps: FeedDataDeps): void {
		this.deps = deps;
	}

	/** Первая порция; догруженные старшие перечитываются от новой границы. */
	async load(): Promise<void> {
		// Первый запрос уже идёт (стартовал при проверке доступа, C95-2) —
		// параллельный вызов ждёт его, а не качает вторую порцию.
		if (this.inFlight) return this.inFlight;
		this.inFlight = this.doLoad().finally(() => {
			this.inFlight = undefined;
		});
		return this.inFlight;
	}

	/**
	 * Первый кадр ленты из снимка на устройстве (C97): экран не ждёт сети.
	 * Сеть идёт своим ходом и заменяет снимок, когда ответит.
	 */
	async loadCached(): Promise<void> {
		// Макет и экран просят снимок оба — второй ждёт тот же, а не пустой ответ:
		// cachedApplied ставится в doLoadCached сразу, до чтения базы.
		if (this.cachedFlight) return this.cachedFlight;
		if (this.cachedApplied || !this.deps.loadCached) return;
		this.cachedFlight = this.doLoadCached().finally(() => {
			this.cachedFlight = undefined;
		});
		return this.cachedFlight;
	}

	private async doLoadCached(): Promise<void> {
		this.cachedApplied = true;
		const snap = await this.deps.loadCached!().catch(() => undefined);
		// Свежий ответ уже на экране — снимок его не перетирает.
		if (!snap || this.freshApplied) return;
		this.nextBefore = snap.next_before;
		this.apply(snap);
		this.loading = false;
	}

	private async doLoad(): Promise<void> {
		this.error = '';
		try {
			const snap = await this.deps.load();
			this.freshApplied = true;
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
			this.apply(snap);
		} catch (err) {
			this.error = isAccessError(err) ? 'Нет доступа' : 'Не удалось загрузить ленту';
		} finally {
			this.loading = false;
		}
	}

	private apply(snap: FeedSnapshot): void {
		this.posts = mergePages(snap.posts, this.olderPosts, (p) => p.id);
		this.feedEvents = snap.events ?? [];
		this.visibleFrom = snap.visible_from ?? null;
		this.circleStartedAt = snap.circle_started_at ?? '';
		this.dividerAt = unreadDividerIndex(this.posts, this.deps.lastRead());
		this.deps.resolveMedia?.(this.posts);
		this.deps.resolveDayMedia?.(this.feedEvents);
	}

	/** Курсор прочитанного догнал (загрузка шла параллельно проверке доступа). */
	redrawDivider(): void {
		this.dividerAt = unreadDividerIndex(this.posts, this.deps.lastRead());
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
