<script lang="ts">
	// Отсечка по дате (6.17, 6.21): с 6.15 — за строкой «Заархивировать и
	// удалить старое» на «Архиве и очистке».
	import { goUp } from '$lib/navigation/up';
	import { goto } from '$app/navigation';
	import { getContext, onMount } from 'svelte';
	import Button from '$ui/forms/Button.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Loading from '$ui/Loading.svelte';
	import Input from '$ui/forms/Input.svelte';
	import Label from '$ui/forms/Label.svelte';
	import VolumeChart from '$ui/forms/VolumeChart.svelte';
	import FormLayout from '$lib/layouts/FormLayout.svelte';
	import { authErrorHint } from '$lib/auth/auth';
	import { isAccessError } from '$lib/api/client';
	import { fetchQuota, type VolumeBucket } from '$lib/circles/settings';
	import { volumeBarCenterX } from '$lib/circles/volumeChart';
	import { formatBytes } from '$lib/format/bytes';
	import { WORD, plural } from '$lib/format/plural';
	import { formatCutoffDate, formatEntryDate } from '$lib/format/time';
	import { CIRCLE_CTX, type CircleContext } from '$lib/journal/context';

	const circle = getContext<CircleContext>(CIRCLE_CTX);

	let quotaBytes = $state(0);
	let volume = $state<VolumeBucket[]>([]);
	let volumeStep = $state<'day' | 'week' | 'month'>('month');
	let cutoffDate = $state('');
	let freedBytes = $state(0);
	let postCount = $state(0);
	// Записи не раньше отсечки — останутся в круге. Нет — сервер старый.
	let keptPosts = $state<number | undefined>(undefined);
	let error = $state('');
	let forbidden = $state(false);
	let loading = $state(true);

	// The instance may run without a storage limit: the API then omits quota_bytes.
	const capped = $derived(quotaBytes > 0);

	// Начало столбика датой. Старый сервер отдавал месяц «2026-09» (C7).
	function bucketStart(period: string): string {
		return period.length === 7 ? `${period}-01` : period;
	}

	function defaultCutoff(): string {
		if (!volume.length) return '';
		return bucketStart(volume[Math.floor(volume.length / 2)].period);
	}

	const STEP_LABEL = { day: 'по дням', week: 'по неделям', month: 'по месяцам' } as const;
	// Концы оси: у дней и недель — даты, у месяцев — годы (по умолчанию графика).
	const axisStart = $derived(
		volumeStep !== 'month' && volume.length ? formatEntryDate(bucketStart(volume[0].period)) : undefined
	);
	const axisEnd = $derived(
		volumeStep !== 'month' && volume.length > 1
			? formatEntryDate(bucketStart(volume[volume.length - 1].period))
			: undefined
	);

	function cutoffLabel(): string {
		if (!cutoffDate) return '';
		return formatCutoffDate(cutoffDate);
	}

	let chartCutoffX = $state(205);
	let cutoffFetchTimer: ReturnType<typeof setTimeout> | undefined;

	function syncChartCutoffX() {
		if (!volume.length || !cutoffDate) return;
		// Последний столбик, что начался не позже отсечки.
		let idx = -1;
		volume.forEach((b, i) => {
			if (bucketStart(b.period) <= cutoffDate) idx = i;
		});
		if (idx >= 0) chartCutoffX = volumeBarCenterX(idx, volume.length);
	}

	function onChartCutoff(index: number) {
		const bucket = volume[index];
		if (!bucket) return;
		cutoffDate = bucketStart(bucket.period);
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
			quotaBytes = data.quota_bytes ?? 0;
			volume = data.volume ?? [];
			volumeStep = data.volume_step ?? 'month';
			postCount = data.post_count;
			keptPosts = data.posts_kept_at_cutoff;
			if (data.freed_at_cutoff_bytes !== undefined) {
				freedBytes = data.freed_at_cutoff_bytes;
			}
			if (!cutoffDate && volume.length) {
				cutoffDate = defaultCutoff();
				// Сразу и счёт под отсечку по умолчанию — иначе подсказка пуста до первого касания.
				void loadQuota(cutoffDate, true);
			}
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
	title="Заархивировать и удалить старое"
	onback={() => goUp(`/circles/${circle.circleId}/quota`)}
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
	{:else if postCount === 0}
		<!-- Без записей нечего ни показать на графике, ни выбрать датой (владелец, 2026-10-23). -->
		<Hint class="gutter">В круге пока нет записей — архивировать нечего.</Hint>
	{:else}
		<Label>Сколько освободится · {STEP_LABEL[volumeStep]}</Label>
		<VolumeChart
			{volume}
			startLabel={axisStart}
			endLabel={axisEnd}
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
		{#if freedBytes || keptPosts !== undefined}
			<!-- 6.17: «Освободится 6,2 ГБ из 10. В круге останется 214 записей из 340.» -->
			<Hint>
				{#if freedBytes}Освободится {formatBytes(freedBytes)}{#if capped}{' '}из {formatBytes(
							quotaBytes
						)}{/if}.{/if}
				{#if keptPosts !== undefined}{' '}В круге останется {plural(keptPosts, WORD.post)} из {postCount}.{/if}
			</Hint>
		{/if}
		<Button class="mt-14" variant="colored" onclick={next}>Дальше: сроки</Button>
	{/if}
</FormLayout>
