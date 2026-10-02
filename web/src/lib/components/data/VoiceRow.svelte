<script lang="ts">
	import Icon from '$ui/Icon.svelte';
	import VoiceWave from '$ui/data/VoiceWave.svelte';
	import { formatDuration } from '$lib/media/record';
	import {
		audioPlayKey,
		audioTrack,
		subscribeAudio,
		toggleAudio,
		type AudioMeta
	} from '$lib/media/audioPlay';

	// Голосовое в записи (4.27): круглая кнопка цвета круга, волна и
	// длительность. Играет через тот же плеер, что звук, — и полосой на
	// других экранах (4.20). Волна закрашивается по мере прослушивания.
	let {
		origin = '',
		blobId,
		peaks = [],
		durationMs = 0,
		meta,
		class: className = ''
	}: {
		origin?: string;
		blobId: string;
		peaks?: number[];
		durationMs?: number;
		meta?: AudioMeta;
		class?: string;
	} = $props();

	let revision = $state(0);
	$effect(() => {
		if (!blobId) return;
		return subscribeAudio(() => {
			revision += 1;
		});
	});

	const track = $derived.by(() => {
		revision;
		return audioTrack(audioPlayKey(origin, blobId));
	});
	const playing = $derived(Boolean(track?.playing || track?.loading));
	const total = $derived(track?.duration ? track.duration * 1000 : durationMs);
	const progress = $derived(track?.duration ? Math.min(1, track.current / track.duration) : 0);
	// Играет — сколько прошло; стоит — сколько длится.
	const label = $derived(
		track?.playing ? formatDuration((track.current ?? 0) * 1000) : formatDuration(total)
	);
	// Старые записи без волны — ровная линия, чтобы строка не пустела.
	const wave = $derived(peaks.length ? peaks : Array.from({ length: 32 }, () => 20));

	function onPlay(e: MouseEvent) {
		e.stopPropagation();
		if (!blobId) return;
		void toggleAudio(origin, blobId, meta);
	}
</script>

<button
	type="button"
	class="voice {className}"
	aria-label={playing ? 'Пауза' : 'Слушать голосовое'}
	aria-pressed={playing ? 'true' : 'false'}
	onclick={onPlay}
>
	<span class="voice-play"><Icon name={playing ? 'pause' : 'play'} /></span>
	<VoiceWave peaks={wave} {progress} />
	<span class="sz">{label}</span>
</button>
