import { goto } from '$app/navigation';
import { page } from '$app/state';
import { formatBytes } from '$lib/format/bytes';
import { audioTimeLabel } from '$lib/journal/present';
import { currentAudio, stopAudio, subscribeAudio, toggleAudio } from './audioPlay';

/** Высота полосы плеера: на столько окно приложения становится короче. */
export const AUDIO_BAR_H = 64;

/** Что показать в полосе — готовые пропы для `$ui/chrome/AudioBar`. */
export interface AudioBarView {
	title: string;
	subtitle: string;
	coverUrl?: string;
	color: string;
	progress: number;
	loading: boolean;
	playing: boolean;
}

/**
 * Полоса плеера (4.20; план 47, 5.4): подписка на играющий звук, где её
 * прятать и `--player-h`. Жило внутри компонента библиотеки — с маршрутом,
 * `goto` и `document`. Звать при создании корневого макета.
 *
 * Полосы нет в ленте круга, где лежит звук, и на экране его записи: там
 * звук виден строкой. Пока полоса видна, окно короче на AUDIO_BAR_H —
 * полоса ввода, плюс и нижние панели встают над ней, а не под неё.
 */
export function useAudioBar() {
	let revision = $state(0);
	$effect(() =>
		subscribeAudio(() => {
			revision += 1;
		})
	);

	const now = $derived.by(() => {
		revision;
		return currentAudio();
	});

	const view = $derived.by((): AudioBarView | null => {
		const meta = now?.meta;
		if (!now || !meta) return null;
		const path = page.url.pathname;
		const circle = `/circles/${meta.circleId}`;
		if (path === circle || path.startsWith(`${circle}/posts/${meta.postId}`)) return null;
		const t = now.track;
		const progress = t.loading
			? t.loading.total
				? Math.min(1, t.loading.received / t.loading.total)
				: 0
			: t.duration
				? Math.min(1, t.current / t.duration)
				: 0;
		const time = t.loading
			? t.loading.total
				? `загрузка · ${formatBytes(t.loading.received)} из ${formatBytes(t.loading.total)}`
				: 'загрузка…'
			: audioTimeLabel(t.started, t.current, t.duration);
		return {
			title: meta.title,
			subtitle: `${meta.circleName} · ${time}`,
			coverUrl: meta.coverUrl,
			color: meta.color,
			progress,
			loading: Boolean(t.loading),
			playing: Boolean(t.playing || t.loading)
		};
	});

	$effect(() => {
		const root = document.documentElement;
		if (view) root.style.setProperty('--player-h', `${AUDIO_BAR_H}px`);
		else root.style.removeProperty('--player-h');
		return () => root.style.removeProperty('--player-h');
	});

	return {
		get view() {
			return view;
		},
		open() {
			const meta = now?.meta;
			if (meta) void goto(`/circles/${meta.circleId}/posts/${meta.postId}`);
		},
		toggle() {
			if (now) void toggleAudio(now.origin, now.blobId, now.meta);
		},
		stop() {
			stopAudio();
		}
	};
}
