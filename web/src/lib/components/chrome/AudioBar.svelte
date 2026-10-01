<script lang="ts">
	import IconButton from '$ui/forms/IconButton.svelte';

	// Полоса плеера (4.20): что играет, откуда, ход, пауза и крестик — внизу
	// любого экрана. Только вид (план 47, 5.4): что играет, когда полосу
	// прятать и высоту окна ведёт `useAudioBar` из `$lib/media/audioBar.svelte`.
	let {
		title,
		subtitle,
		coverUrl,
		color,
		progress,
		loading = false,
		playing,
		onopen,
		ontoggle,
		onstop
	}: {
		title: string;
		/** «Дача · 1:12 из 3:40» или «загрузка · …». */
		subtitle: string;
		coverUrl?: string;
		/** Цвет круга: ход и кнопка паузы. */
		color: string;
		/** 0…1 — сыграно или скачано. */
		progress: number;
		/** Файл ещё качается — ход серый. */
		loading?: boolean;
		playing: boolean;
		/** Открыть запись со звуком. */
		onopen: () => void;
		ontoggle: () => void;
		onstop: () => void;
	} = $props();
</script>

<div class="audio-bar" role="region" aria-label="Звук">
	<div class="audio-bar-line" class:loading>
		<i style:width="{progress * 100}%" style:background={loading ? undefined : color}></i>
	</div>
	<div class="audio-bar-row">
		<button type="button" class="audio-bar-main" onclick={onopen}>
			<span class="pic audio-bar-cover">{#if coverUrl}<img src={coverUrl} alt="" />{/if}</span>
			<span class="audio-bar-text">
				<span class="audio-bar-title">{title}</span>
				<span class="audio-bar-sub">{subtitle}</span>
			</span>
		</button>
		<IconButton
			name={playing ? 'pause' : 'play'}
			label={playing ? 'Пауза' : 'Слушать'}
			style="color:{color}"
			onclick={ontoggle}
		/>
		<IconButton name="x" label="Остановить" onclick={onstop} />
	</div>
</div>
