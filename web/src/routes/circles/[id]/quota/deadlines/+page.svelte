<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import { getContext } from 'svelte';
	import Button from '$ui/forms/Button.svelte';
	import Chip from '$ui/forms/Chip.svelte';
	import ChipGroup from '$ui/forms/ChipGroup.svelte';
	import DangerNote from '$ui/forms/DangerNote.svelte';
	import DangerZone from '$ui/forms/DangerZone.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Input from '$ui/forms/Input.svelte';
	import Label from '$ui/forms/Label.svelte';
	import SettingsRow from '$ui/data/SettingsRow.svelte';
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

	let cutoffDate = $state(
		circle.archiveCycle?.active && circle.archiveCycle.cutoff_date
			? circle.archiveCycle.cutoff_date
			: ($page.url.searchParams.get('cutoff') ?? '')
	);
	let deadline = $state(
		circle.archiveCycle?.active && circle.archiveCycle.deadline
			? circle.archiveCycle.deadline.slice(0, 10)
			: ''
	);
	let reminderSec = $state(
		circle.archiveCycle?.active
			? (circle.archiveCycle.reminder_before_sec ?? REMINDER_OPTIONS[1].sec)
			: REMINDER_OPTIONS[1].sec
	);
	let cutoffStats = $state('');
	let cutoffLocked = $state(Boolean(circle.archiveCycle?.cutoff_locked));
	const cutoffInputId = 'quota-cutoff-date';
	let error = $state('');
	let loading = $state(false);

	const showReminderChips = $derived(!activeCycle || cutoffLocked);
	const reminderReadOnly = $derived(activeCycle && cutoffLocked);

	const cutoffTitle = $derived(cutoffDate ? formatEntryDate(cutoffDate) : '…');

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

	/** Самый ранний срок — завтра: сервер требует не меньше суток (ARC-7). */
	function minDeadline(): string {
		const d = new Date();
		d.setDate(d.getDate() + 1);
		return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
	}

	async function launch() {
		if (!cutoffDate || !deadline) return;
		if (deadline < minDeadline()) {
			error = 'Срок — не раньше завтрашнего дня: участникам нужно время скачать архив';
			return;
		}
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

	$effect(() => {
		if (activeCycle && cycle) {
			cutoffDate = cycle.cutoff_date;
			deadline = cycle.deadline.slice(0, 10);
			cutoffLocked = cycle.cutoff_locked;
			reminderSec = cycle.reminder_before_sec ?? REMINDER_OPTIONS[1].sec;
			void loadCutoffStats(cycle.cutoff_date);
		} else if (cutoffParam) {
			cutoffDate = cutoffParam;
			if (!deadline) deadline = defaultDeadline();
			void loadCutoffStats(cutoffParam);
		} else if (!cutoffDate) {
			goto(`/circles/${circle.circleId}/quota`);
		}
	});
</script>

<FormLayout app color={circle.color} title="Сроки архивации" onback={goBack}>
	<Label>Отсечка</Label>
	{#if activeCycle && cutoffLocked}
		<SettingsRow
			title={cutoffTitle}
			subtitle={cutoffStats}
			value="дата замерла"
			chevron={false}
			style="border-top:1px solid var(--line);border-bottom:1px solid var(--line)"
		/>
		<Hint
			>Кто-то уже скачал архив. Эту отсечку больше не сдвинуть: другой диапазон — новый цикл,
			и все качают заново.</Hint
		>
	{:else if activeCycle}
		<SettingsRow
			title={cutoffTitle}
			subtitle={cutoffStats}
			value="изменить"
			chevron={false}
			style="border-top:1px solid var(--line);border-bottom:1px solid var(--line)"
			onclick={() => {
				const el = document.getElementById(cutoffInputId) as HTMLInputElement | null;
				el?.showPicker?.() ?? el?.focus();
			}}
		/>
	{/if}
	{#if !activeCycle || !cutoffLocked}
		<Input
			id={cutoffInputId}
			active
			type="date"
			bind:value={cutoffDate}
			disabled={cutoffLocked}
			onchange={() => void loadCutoffStats(cutoffDate)}
		/>
	{/if}
	{#if !activeCycle || !cutoffLocked}
		<Hint
			>Отсечка двигается, пока никто не скачал архив. После первого скачивания она замирает:
			другой диапазон — это новый цикл, а не правка этого.</Hint
		>
	{/if}
	<Label>Скачать до</Label>
	<Input active type="date" min={minDeadline()} bind:value={deadline} />
	<Hint
		>Срок двигается в любую сторону и в любой момент: архив снят на отсечку, а не на срок, и от
		сдвига не портится.</Hint
	>
	{#if showReminderChips}
		<Label>Напомнить письмом</Label>
		<ChipGroup>
			{#each REMINDER_OPTIONS as opt (opt.sec)}
				<Chip
					selected={reminderSec === opt.sec}
					onclick={reminderReadOnly ? undefined : () => (reminderSec = opt.sec)}
				>
					{opt.label}
				</Chip>
			{/each}
		</ChipGroup>
		<Hint
			>Писем два: на старте и напоминание. Сдвинете срок — напоминание пересчитается от новой даты,
			интервал останется прежним.</Hint
		>
	{/if}
	{#if !activeCycle}
		<DangerNote class="mt-20">
			{deadline ? formatEntryDate(deadline) : '…'} всё до {cutoffDate
				? formatEntryDate(cutoffDate)
				: '…'} удалится с сервера — независимо от того, все ли успели скачать. Предупредить людей —
			на вас.
		</DangerNote>
	{:else if cutoffLocked}
		<DangerZone
			style="margin-top:20px"
			items={['Новый цикл архивации']}
			onitem={() => goto(`/circles/${circle.circleId}/quota`)}
		/>
		<Hint class="mt-8"
			>Начнёте заново — уже скачанные архивы останутся у людей, но отсечка и срок будут другими, и
			качать нужно снова.</Hint
		>
	{/if}
	<Button class="mt-14" variant="colored" {loading} onclick={() => void launch()}>
		{activeCycle ? 'Сохранить сроки' : 'Запустить архивацию'}
	</Button>
	{#if error}
		<Hint class="gutter">{error}</Hint>
	{/if}
</FormLayout>
