<script lang="ts">
	import { goUp } from '$lib/navigation/up';
	import SizeField from '$ui/forms/SizeField.svelte';
	import { goto } from '$app/navigation';
	import { getContext, onMount } from 'svelte';
	import Button from '$ui/forms/Button.svelte';
	import Chip from '$ui/forms/Chip.svelte';
	import ChipGroup from '$ui/forms/ChipGroup.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Loading from '$ui/Loading.svelte';
	import Label from '$ui/forms/Label.svelte';
	import FormLayout from '$lib/layouts/FormLayout.svelte';
	import { authErrorHint } from '$lib/auth/auth';
	import {
		QUOTA_CHIPS,
		QUOTA_GB,
		fetchQuota,
		requestQuotaExpansion,
		type QuotaChipKey
	} from '$lib/circles/settings';
	import { formatBytes } from '$lib/format/bytes';
	import { CIRCLE_CTX, type CircleContext } from '$lib/journal/context';

	const circle = getContext<CircleContext>(CIRCLE_CTX);

	let usedBytes = $state(0);
	let quotaBytes = $state(0);
	let chip = $state<QuotaChipKey>('10');
	let customBytes = $state(20 * QUOTA_GB);
	let loading = $state(false);
	let pageLoading = $state(true);
	let error = $state('');

	// Нижняя граница — уже занятое, как сказано под полем (6.18): «Ниже этого
	// числа просить незачем». Меньше нынешней квоты попросить можно (C35):
	// раньше экран требовал больше квоты, хотя подпись этого не говорила.
	const requestFloor = $derived(usedBytes);
	const selectedBytes = $derived.by(() => {
		if (chip === 'custom') {
			// Своё — в мегабайтах или гигабайтах (поле с чипами «МБ / ГБ»).
			const bytes = customBytes;
			if (bytes <= 0 || bytes <= requestFloor || bytes > 1024 * QUOTA_GB) return null;
			return bytes;
		}
		const picked = QUOTA_CHIPS.find((c) => c.key === chip);
		const bytes = picked?.bytes;
		if (bytes == null || bytes <= requestFloor) return null;
		return bytes;
	});

	function syncChip(bytes: number) {
		if (bytes === 5 * QUOTA_GB && bytes >= usedBytes) {
			chip = '5';
			return;
		}
		if (bytes === 10 * QUOTA_GB && bytes >= usedBytes) {
			chip = '10';
			return;
		}
		chip = 'custom';
		customBytes = bytes;
	}

	function chipDisabled(key: QuotaChipKey): boolean {
		if (key === 'none' || key === 'custom') return false;
		const picked = QUOTA_CHIPS.find((c) => c.key === key);
		return picked?.bytes != null && picked.bytes <= requestFloor;
	}

	async function submit() {
		if (selectedBytes == null) {
			error =
				requestFloor > 0
					? `Укажите больше ${formatBytes(requestFloor)}`
					: 'Укажите, сколько нужно';
			return;
		}
		loading = true;
		error = '';
		try {
			await requestQuotaExpansion(circle.origin, circle.circleId, selectedBytes);
			goto(`/circles/${circle.circleId}/quota`);
		} catch (err) {
			error = authErrorHint(err);
			loading = false;
		}
	}

	onMount(async () => {
		try {
			const data = await fetchQuota(circle.origin, circle.circleId);
			usedBytes = data.used_bytes;
			quotaBytes = data.quota_bytes ?? 0;
			const suggested = Math.max(usedBytes, quotaBytes);
			const next =
				suggested >= 10 * QUOTA_GB
					? Math.min(1024, Math.ceil((suggested / QUOTA_GB) * 1.5)) * QUOTA_GB
					: 10 * QUOTA_GB;
			syncChip(Math.max(next, usedBytes + QUOTA_GB));
		} catch (err) {
			error = authErrorHint(err);
		} finally {
			pageLoading = false;
		}
	});
</script>

<FormLayout
	app
	color={circle.color}
	title="Попросить место"
	onback={() => goUp(`/circles/${circle.circleId}/quota`)}
>
	{#if pageLoading}
		<Loading />
	{:else}
		<Hint class="gutter">
			{#if quotaBytes}
				Сейчас у «{circle.name}» {formatBytes(quotaBytes)} — свою квоту ставит администратор
				сервера.
			{:else}
				У «{circle.name}» ограничение не задано — администратор сервера может выделить место.
			{/if}
			Шаг необязательный: можно вернуться и сразу выбрать отсечку.
		</Hint>
		<Label>Сколько нужно</Label>
		<ChipGroup>
			{#each QUOTA_CHIPS.filter((c) => c.key !== 'none') as opt (opt.key)}
				<Chip
					selected={chip === opt.key}
					disabled={chipDisabled(opt.key)}
					onclick={() => {
						chip = opt.key;
						if (opt.key !== 'custom' && opt.bytes != null) {
							customBytes = opt.bytes;
						}
					}}
				>
					{opt.label}
				</Chip>
			{/each}
		</ChipGroup>
		{#if chip === 'custom'}
			<SizeField bind:bytes={customBytes} />
		{/if}
		<Hint class="mt-10">
			{formatBytes(usedBytes)} уже лежит. Ниже этого числа просить незачем — место всё равно
			кончится.
		</Hint>
		<Button class="mt-16" variant="colored" {loading} onclick={() => void submit()}>
			Отправить запрос
		</Button>
		<Hint class="mt-18" centered
			>Отказ ничего не ломает: цикл архивации останется на вас.</Hint
		>
		{#if error}
			<Hint class="gutter">{error}</Hint>
		{/if}
	{/if}
</FormLayout>
