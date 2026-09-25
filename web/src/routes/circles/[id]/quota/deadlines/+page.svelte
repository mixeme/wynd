<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import { getContext, onMount } from 'svelte';
	import Button from '$ui/forms/Button.svelte';
	import Chip from '$ui/forms/Chip.svelte';
	import ChipGroup from '$ui/forms/ChipGroup.svelte';
	import DangerNote from '$ui/forms/DangerNote.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Input from '$ui/forms/Input.svelte';
	import Label from '$ui/forms/Label.svelte';
	import FormLayout from '$lib/layouts/FormLayout.svelte';
	import { authErrorHint } from '$lib/auth/auth';
	import {
		REMINDER_OPTIONS,
		fetchQuota,
		moveCutoff,
		moveDeadline,
		startArchiveCycle
	} from '$lib/circles/settings';
	import { formatBytes } from '$lib/format/bytes';
	import { formatEntryDate } from '$lib/format/time';
	import { CIRCLE_CTX, type CircleContext } from '$lib/journal/context';

	const circle = getContext<CircleContext>(CIRCLE_CTX);

	const cutoffParam = $derived($page.url.searchParams.get('cutoff') ?? '');
	const cycle = $derived(circle.archiveCycle);
	const activeCycle = $derived(Boolean(cycle?.active));

	let cutoffDate = $state('');
	let deadline = $state('');
	let reminderSec = $state(REMINDER_OPTIONS[1].sec);
	let cutoffStats = $state('');
	let cutoffLocked = $state(false);
	let error = $state('');
	let loading = $state(false);

	function defaultDeadline(): string {
		const d = new Date();
		d.setMonth(d.getMonth() + 1);
		return d.toISOString().slice(0, 10);
	}

	async function loadCutoffStats(date: string) {
		if (!date) return;
		try {
			const q = await fetchQuota(circle.origin, circle.circleId, date);
			const freed = q.freed_at_cutoff_bytes ?? 0;
			cutoffStats = freed ? `${formatBytes(freed)}` : '';
		} catch {
			cutoffStats = '';
		}
	}

	async function launch() {
		if (!cutoffDate || !deadline) return;
		loading = true;
		error = '';
		try {
			const deadlineIso = new Date(`${deadline}T23:59:59`).toISOString();
			if (activeCycle) {
				if (!cutoffLocked && cutoffDate !== cycle?.cutoff_date) {
					await moveCutoff(circle.origin, circle.circleId, cutoffDate);
				}
				await moveDeadline(circle.origin, circle.circleId, deadlineIso);
				await circle.refresh();
				goto(`/circles/${circle.circleId}/settings`);
			} else {
				await startArchiveCycle(circle.origin, circle.circleId, {
					cutoff_date: cutoffDate,
					deadline: deadlineIso,
					reminder_before_sec: reminderSec
				});
				goto(`/circles/${circle.circleId}`);
			}
		} catch (err) {
			error = authErrorHint(err);
			loading = false;
		}
	}

	function goBack() {
		if (activeCycle) {
			goto(`/circles/${circle.circleId}/settings`);
		} else {
			goto(`/circles/${circle.circleId}/quota`);
		}
	}

	onMount(() => {
		if (activeCycle && cycle) {
			cutoffDate = cycle.cutoff_date;
			deadline = cycle.deadline.slice(0, 10);
			cutoffLocked = cycle.cutoff_locked;
			reminderSec = cycle.reminder_before_sec ?? REMINDER_OPTIONS[1].sec;
			void loadCutoffStats(cutoffDate);
		} else {
			cutoffDate = cutoffParam;
			deadline = defaultDeadline();
			if (!cutoffParam) goto(`/circles/${circle.circleId}/quota`);
			void loadCutoffStats(cutoffDate);
		}
	});
</script>

<FormLayout app color={circle.color} title="Сроки архивации" onback={goBack}>
	<Label>Отсечка</Label>
	{#if activeCycle}
		<div class="row2" style="border-top:1px solid var(--line);border-bottom:1px solid var(--line)">
			<div class="g">
				<div style="font-weight:600">{formatEntryDate(cutoffDate)}</div>
				{#if cutoffStats}
					<div class="sub">{cutoffStats}</div>
				{/if}
			</div>
			{#if !cutoffLocked}
				<span class="val">изменить</span>
			{/if}
		</div>
	{/if}
	{#if !activeCycle || !cutoffLocked}
		<Input
			active
			type="date"
			bind:value={cutoffDate}
			disabled={cutoffLocked}
			onchange={() => void loadCutoffStats(cutoffDate)}
		/>
	{/if}
	<Hint
		>Отсечка двигается, пока никто не скачал архив. После первого скачивания она замирает: другой
		диапазон — это новый цикл, а не правка этого.</Hint
	>
	<Label style="margin-top:18px">Скачать до</Label>
	<Input active type="date" bind:value={deadline} />
	<Hint
		>Срок двигается в любую сторону и в любой момент: архив снят на отсечку, а не на срок, и от
		сдвига не портится.</Hint
	>
	{#if !activeCycle}
		<Label style="margin-top:18px">Напомнить письмом</Label>
		<ChipGroup>
			{#each REMINDER_OPTIONS as opt (opt.sec)}
				<Chip selected={reminderSec === opt.sec} onclick={() => (reminderSec = opt.sec)}>
					{opt.label}
				</Chip>
			{/each}
		</ChipGroup>
		<Hint
			>Писем два: на старте и напоминание. Сдвинете срок — напоминание пересчитается от новой даты,
			интервал останется прежним.</Hint
		>
	{/if}
	<DangerNote style="margin-top:20px">
		{deadline ? formatEntryDate(deadline) : '…'} всё до {cutoffDate
			? formatEntryDate(cutoffDate)
			: '…'} удалится с сервера — независимо от того, все ли успели скачать. Предупредить людей —
		на вас.
	</DangerNote>
	<Button variant="colored" style="margin-top:14px" {loading} onclick={() => void launch()}>
		{activeCycle ? 'Сохранить сроки' : 'Запустить архивацию'}
	</Button>
	{#if error}
		<Hint style="margin:16px">{error}</Hint>
	{/if}
</FormLayout>
