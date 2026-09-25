<script lang="ts">
	import { goto } from '$app/navigation';
	import { getContext, onMount } from 'svelte';
	import Button from '$ui/forms/Button.svelte';
	import Chip from '$ui/forms/Chip.svelte';
	import ChipGroup from '$ui/forms/ChipGroup.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Input from '$ui/forms/Input.svelte';
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
	let customGb = $state('20');
	let loading = $state(false);
	let pageLoading = $state(true);
	let error = $state('');

	const minGb = $derived(Math.max(1, Math.ceil(usedBytes / QUOTA_GB)));
	const requestFloor = $derived(quotaBytes > 0 ? Math.max(usedBytes, quotaBytes) : usedBytes);
	const selectedBytes = $derived.by(() => {
		if (chip === 'custom') {
			const n = Number(customGb.trim().replace(',', '.'));
			if (!Number.isInteger(n) || n < minGb || n > 1024) return null;
			const bytes = n * QUOTA_GB;
			if (bytes <= requestFloor) return null;
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
		customGb = String(Math.max(minGb, Math.round(bytes / QUOTA_GB)));
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
					: `Укажите не меньше ${minGb} ГБ — столько уже занято`;
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
	onback={() => goto(`/circles/${circle.circleId}/quota`)}
>
	{#if pageLoading}
		<Hint style="margin:16px">Загрузка…</Hint>
	{:else}
		<Hint style="margin:16px">
			{#if quotaBytes}
				Сейчас у «{circle.name}» {formatBytes(quotaBytes)} — свою квоту ставит администратор
				сервера.
			{:else}
				У «{circle.name}» ограничение не задано — администратор сервера может выделить место.
			{/if}
			Шаг необязательный: можно вернуться и сразу выбрать отсечку.
		</Hint>
		<Label style="margin-top:18px">Сколько нужно</Label>
		<ChipGroup>
			{#each QUOTA_CHIPS.filter((c) => c.key !== 'none') as opt (opt.key)}
				<Chip
					selected={chip === opt.key}
					disabled={chipDisabled(opt.key)}
					onclick={() => {
						chip = opt.key;
						if (opt.key !== 'custom' && opt.bytes != null) {
							customGb = String(Math.round(opt.bytes / QUOTA_GB));
						}
					}}
				>
					{opt.label}
				</Chip>
			{/each}
		</ChipGroup>
		{#if chip === 'custom'}
			<div class="rowin" style="margin-top:10px;align-items:center">
				<Input
					active
					type="number"
					min={String(minGb)}
					max="1024"
					bind:value={customGb}
					style="width:72px;margin:0"
				/>
				<span class="hint" style="margin:0">ГБ</span>
			</div>
		{/if}
		<Hint style="margin-top:10px">
			{formatBytes(usedBytes)} уже лежит. Ниже этого числа просить незачем — место всё равно
			кончится.
		</Hint>
		<Button variant="colored" style="margin-top:16px" {loading} onclick={() => void submit()}>
			Отправить запрос
		</Button>
		<Hint centered style="margin-top:18px"
			>Отказ ничего не ломает: цикл архивации останется на вас.</Hint
		>
		{#if error}
			<Hint style="margin:16px">{error}</Hint>
		{/if}
	{/if}
</FormLayout>
