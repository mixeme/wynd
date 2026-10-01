<script lang="ts">
	import { goto } from '$app/navigation';
	import { getContext, onMount } from 'svelte';
	import Button from '$ui/forms/Button.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Loading from '$ui/Loading.svelte';
	import Input from '$ui/forms/Input.svelte';
	import Label from '$ui/forms/Label.svelte';
	import Meter from '$ui/forms/Meter.svelte';
	import SettingsRow from '$ui/data/SettingsRow.svelte';
	import VolumeChart from '$ui/forms/VolumeChart.svelte';
	import FormLayout from '$lib/layouts/FormLayout.svelte';
	import { authErrorHint } from '$lib/auth/auth';
	import { isAccessError } from '$lib/api/client';
	import { fetchQuota, type VolumeBucket } from '$lib/circles/settings';
	import { volumeBarCenterX } from '$lib/circles/volumeChart';
	import { formatBytes } from '$lib/format/bytes';
	import { WORD, plural } from '$lib/format/plural';
	import { formatEntryDate } from '$lib/format/time';
	import { CIRCLE_CTX, type CircleContext } from '$lib/journal/context';

	const circle = getContext<CircleContext>(CIRCLE_CTX);

	let usedBytes = $state(0);
	let quotaBytes = $state(0);
	let volume = $state<VolumeBucket[]>([]);
	let cutoffDate = $state('');
	let freedBytes = $state(0);
	let medianPostBytes = $state(0);
	let error = $state('');
	let forbidden = $state(false);
	let loading = $state(true);

	const usedGb = $derived(usedBytes / (1024 * 1024 * 1024));
	const quotaGb = $derived(Math.max(quotaBytes / (1024 * 1024 * 1024), 0.001));
	const full = $derived(quotaBytes > 0 && usedBytes >= quotaBytes);
	// The instance may run without a storage limit: the API then omits quota_bytes.
	const capped = $derived(quotaBytes > 0);

	const postsEstimate = $derived.by(() => {
		if (!volume.length || !medianPostBytes || !capped) return 0;
		const remaining = Math.max(quotaBytes - usedBytes, 0);
		return Math.floor(remaining / medianPostBytes);
	});

	function defaultCutoff(): string {
		if (!volume.length) return '';
		return `${volume[Math.floor(volume.length / 2)].period}-01`;
	}

	function cutoffLabel(): string {
		if (!cutoffDate) return '';
		return formatEntryDate(cutoffDate);
	}

	let chartCutoffX = $state(205);
	let cutoffFetchTimer: ReturnType<typeof setTimeout> | undefined;

	function syncChartCutoffX() {
		if (!volume.length || !cutoffDate) return;
		const period = cutoffDate.slice(0, 7);
		const idx = volume.findIndex((b) => b.period === period);
		if (idx >= 0) chartCutoffX = volumeBarCenterX(idx, volume.length);
	}

	function onChartCutoff(index: number) {
		const bucket = volume[index];
		if (!bucket) return;
		cutoffDate = `${bucket.period}-01`;
		clearTimeout(cutoffFetchTimer);
		cutoffFetchTimer = setTimeout(() => {
			void loadQuota(cutoffDate, true);
		}, 180);
	}

	async function loadQuota(cutoff?: string, quiet = false) {
		if (!quiet) loading = true;
		error = '';
		try {
			const data = await fetchQuota(circle.origin, circle.circleId, cutoff);
			usedBytes = data.used_bytes;
			quotaBytes = data.quota_bytes ?? 0;
			volume = data.volume ?? [];
			medianPostBytes = data.median_post_bytes ?? 0;
			if (data.freed_at_cutoff_bytes !== undefined) {
				freedBytes = data.freed_at_cutoff_bytes;
			}
			if (!cutoffDate && volume.length) cutoffDate = defaultCutoff();
			syncChartCutoffX();
		} catch (err) {
			if (isAccessError(err)) {
				forbidden = true;
				error = 'Управлять местом может только владелец круга';
			} else {
				error = authErrorHint(err);
			}
		} finally {
			if (!quiet) loading = false;
		}
	}

	async function onCutoffChange(date: string) {
		cutoffDate = date;
		syncChartCutoffX();
		await loadQuota(date, volume.length > 0);
	}

	function next() {
		if (!cutoffDate) return;
		goto(`/circles/${circle.circleId}/quota/deadlines?cutoff=${cutoffDate}`);
	}

	onMount(() => {
		void loadQuota();
		return () => clearTimeout(cutoffFetchTimer);
	});
</script>

<FormLayout
	app
	color={circle.color}
	title="Архив и очистка"
	onback={() => goto(`/circles/${circle.circleId}/settings`)}
>
	{#if loading}
		<Loading />
	{:else if error}
		<Hint class="gutter">{error}</Hint>
		{#if forbidden}
			<Button class="mt-8" variant="ghost" onclick={() => goto(`/circles/${circle.circleId}/settings`)}>
				К настройкам
			</Button>
		{/if}
	{:else}
		<Label>Место</Label>
		<Meter value={capped ? usedGb : 0} max={quotaGb} />
		<Hint class="mt-8">
			{#if capped}
				{formatBytes(usedBytes)} из {formatBytes(quotaBytes)}
			{:else}
				{formatBytes(usedBytes)} · ограничение не задано
			{/if}
			{#if full}· фотографии больше не загружаются, текст пишется{/if}
		</Hint>
		{#if capped}
			<SettingsRow
				title="Попросить у администратора"
				subtitle="шаг необязательный — можно сразу к отсечке"
				style="margin-top:10px;border-top:1px solid var(--line);border-bottom:1px solid var(--line)"
				onclick={() => goto(`/circles/${circle.circleId}/quota/request`)}
			/>
		{/if}
		<Label>Сколько освободит отсечка</Label>
		<VolumeChart
			{volume}
			cutoffLabel={cutoffLabel()}
			bind:cutoffX={chartCutoffX}
			oncutoff={onChartCutoff}
		/>
		<Label class="mt-14">Архивировать всё до</Label>
		<Input
			active
			type="date"
			bind:value={cutoffDate}
			onchange={() => void onCutoffChange(cutoffDate)}
		/>
		{#if freedBytes || postsEstimate}
			<Hint>
				{#if freedBytes}Освободится {formatBytes(freedBytes)}{#if capped} из {formatBytes(quotaBytes)}{/if}.{/if}
				{#if postsEstimate} Останется ~{plural(postsEstimate, WORD.post)}.{/if}
			</Hint>
		{/if}
		<Button class="mt-14" variant="colored" onclick={next}>Дальше: сроки</Button>
	{/if}
</FormLayout>
