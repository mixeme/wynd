<script lang="ts">
	import { goUp } from '$lib/navigation/up';
	import { goto } from '$app/navigation';
	import { getContext, onMount } from 'svelte';
	import Button from '$ui/forms/Button.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Loading from '$ui/Loading.svelte';
	import Label from '$ui/forms/Label.svelte';
	import Meter from '$ui/forms/Meter.svelte';
	import SettingsRow from '$ui/data/SettingsRow.svelte';
	import FormLayout from '$lib/layouts/FormLayout.svelte';
	import { authErrorHint } from '$lib/auth/auth';
	import { isAccessError } from '$lib/api/client';
	import { fetchQuota, type QuotaRequestInfo } from '$lib/circles/settings';
	import { formatBytes } from '$lib/format/bytes';
	import { formatSessionDay } from '$lib/format/time';
	import { CIRCLE_CTX, type CircleContext } from '$lib/journal/context';

	// «Архив и очистка» списком (6.31): место и просьба о нём, освободить
	// место отсечкой, полный архив круга.
	const circle = getContext<CircleContext>(CIRCLE_CTX);

	let usedBytes = $state(0);
	let quotaBytes = $state(0);
	let lastRequest = $state<QuotaRequestInfo | undefined>(undefined);
	let error = $state('');
	let forbidden = $state(false);
	let loading = $state(true);

	const usedGb = $derived(usedBytes / (1024 * 1024 * 1024));
	const quotaGb = $derived(Math.max(quotaBytes / (1024 * 1024 * 1024), 0.001));
	const full = $derived(quotaBytes > 0 && usedBytes >= quotaBytes);
	// The instance may run without a storage limit: the API then omits quota_bytes.
	const capped = $derived(quotaBytes > 0);

	async function load() {
		loading = true;
		error = '';
		try {
			const data = await fetchQuota(circle.origin, circle.circleId);
			usedBytes = data.used_bytes;
			quotaBytes = data.quota_bytes ?? 0;
			lastRequest = data.last_request;
		} catch (err) {
			if (isAccessError(err)) {
				forbidden = true;
				error = 'Управлять местом может только владелец круга';
			} else {
				error = authErrorHint(err);
			}
		} finally {
			loading = false;
		}
	}

	onMount(() => void load());
</script>

<FormLayout
	app
	color={circle.color}
	title="Архив и очистка"
	onback={() => goUp(`/circles/${circle.circleId}/settings`)}
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
			{#if full}· писать в круг больше нельзя{/if}
		</Hint>
		{#if lastRequest?.status === 'pending'}
			<!-- Просьба отправлена (6.44): одна, пока не ответили. -->
			<SettingsRow
				title={capped
					? `Попросили ${formatBytes(lastRequest.requested_bytes)} вместо ${formatBytes(quotaBytes)}`
					: `Попросили ${formatBytes(lastRequest.requested_bytes)}`}
				subtitle={`${formatSessionDay(lastRequest.created_at)} · ждём ответа администратора, он придёт письмом`}
				chevron={false}
				class="mt-10"
				divided="both"
			/>
		{:else if capped}
			<!-- Отказ (6.45) виден до новой просьбы или отсечки. -->
			<SettingsRow
				title="Попросить у администратора"
				subtitle={lastRequest?.status === 'rejected'
					? `${formatSessionDay(lastRequest.resolved_at ?? lastRequest.created_at)} администратор отказал в ${formatBytes(lastRequest.requested_bytes)}. Можно попросить ещё раз`
					: 'больше места для круга'}
				class="mt-10"
				divided="both"
				onclick={() => goto(`/circles/${circle.circleId}/quota/request`)}
			/>
		{/if}
		<Label class="mt-22">Освободить место</Label>
		<SettingsRow
			title="Заархивировать и удалить старое"
			subtitle="записи до выбранной даты: каждый скачает свой архив, потом они удалятся"
			divided="both"
			onclick={() => goto(`/circles/${circle.circleId}/quota/cleanup`)}
		/>
		<Label class="mt-22">Полный архив круга</Label>
		<SettingsRow
			title="Скачать"
			subtitle="один файл со всеми записями и фотографиями"
			divided="both"
			onclick={() => goto(`/circles/${circle.circleId}/quota/full`)}
		/>
	{/if}
</FormLayout>
