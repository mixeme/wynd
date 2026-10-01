<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import IconButton from '$ui/forms/IconButton.svelte';
	import { formatBytes } from '$lib/format/bytes';
	import { audioTimeLabel } from '$lib/journal/present';
	import { currentAudio, stopAudio, subscribeAudio, toggleAudio } from '$lib/media/audioPlay';

	// Полоса плеера (4.20): звук не обрывается, когда уходите с экрана, где он
	// лежит. Внизу любого экрана — что играет, откуда, пауза и крестик. На
	// экране самой записи и в ленте её круга полосы нет: там звук виден строкой.
	// Пока полоса видна, окно приложения короче на её высоту (--player-h):
	// полоса ввода, плюс и нижние панели встают над ней, а не под неё.

	const BAR_H = 64;

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

	const visible = $derived.by(() => {
		const meta = now?.meta;
		if (!now || !meta) return false;
		const path = page.url.pathname;
		const circle = `/circles/${meta.circleId}`;
		return path !== circle && !path.startsWith(`${circle}/posts/${meta.postId}`);
	});

	$effect(() => {
		const root = document.documentElement;
		if (visible) root.style.setProperty('--player-h', `${BAR_H}px`);
		else root.style.removeProperty('--player-h');
		return () => root.style.removeProperty('--player-h');
	});

	const progress = $derived.by(() => {
		const t = now?.track;
		if (!t) return 0;
		if (t.loading) return t.loading.total ? Math.min(1, t.loading.received / t.loading.total) : 0;
		return t.duration ? Math.min(1, t.current / t.duration) : 0;
	});

	const timeLabel = $derived.by(() => {
		const t = now?.track;
		if (!t) return '';
		if (t.loading) {
			return t.loading.total
				? `загрузка · ${formatBytes(t.loading.received)} из ${formatBytes(t.loading.total)}`
				: 'загрузка…';
		}
		return audioTimeLabel(t.started, t.current, t.duration);
	});

	const playing = $derived(Boolean(now?.track.playing || now?.track.loading));

	function openPost() {
		const meta = now?.meta;
		if (meta) goto(`/circles/${meta.circleId}/posts/${meta.postId}`);
	}

	function toggle() {
		if (now) void toggleAudio(now.origin, now.blobId, now.meta);
	}
</script>

{#if visible && now?.meta}
	<div class="audio-bar" role="region" aria-label="Звук">
		<div class="audio-bar-line" class:loading={Boolean(now.track.loading)}>
			<i style:width="{progress * 100}%" style:background={now.track.loading ? undefined : now.meta.color}
			></i>
		</div>
		<div class="audio-bar-row">
			<button type="button" class="audio-bar-main" onclick={openPost}>
				<span class="pic audio-bar-cover"
					>{#if now.meta.coverUrl}<img src={now.meta.coverUrl} alt="" />{/if}</span
				>
				<span class="audio-bar-text">
					<span class="audio-bar-title">{now.meta.title}</span>
					<span class="audio-bar-sub">{now.meta.circleName} · {timeLabel}</span>
				</span>
			</button>
			<IconButton
				name={playing ? 'pause' : 'play'}
				label={playing ? 'Пауза' : 'Слушать'}
				style="color:{now.meta.color}"
				onclick={toggle}
			/>
			<IconButton name="x" label="Остановить" onclick={() => stopAudio()} />
		</div>
	</div>
{/if}
