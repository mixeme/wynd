<script lang="ts">
	import { goto } from '$app/navigation';
	import { getContext } from 'svelte';
	import Button from '$ui/forms/Button.svelte';
	import Chip from '$ui/forms/Chip.svelte';
	import ChipGroup from '$ui/forms/ChipGroup.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Label from '$ui/forms/Label.svelte';
	import SettingsRow from '$ui/data/SettingsRow.svelte';
	import FormLayout from '$lib/layouts/FormLayout.svelte';
	import { formatBytes } from '$lib/format/bytes';
	import { formatDeadline, formatEntryDate, daysUntil } from '$lib/format/time';
	import { downloadArchive } from '$lib/journal/posts';
	import { CIRCLE_CTX, type CircleContext } from '$lib/journal/context';

	const circle = getContext<CircleContext>(CIRCLE_CTX);

	let layout = $state<'feed' | 'posts'>('feed');

	const cycle = $derived(circle.archiveCycle);

	function goBack() {
		goto(`/circles/${circle.circleId}/settings`);
	}

	function download() {
		if (cycle?.download_url) {
			void downloadArchive(circle.origin, cycle.download_url, layout);
		}
	}
</script>

<FormLayout app color={circle.color} title="Архив круга" onback={goBack}>
	{#if !cycle?.active}
		<Hint style="margin:16px">Сейчас архивация не идёт.</Hint>
	{:else}
		<Label>Что внутри</Label>
		<SettingsRow
			icon="photo"
			title="/media"
			subtitle="фото и видео записей"
			chevron={false}
			style="border-top:1px solid var(--line)"
		/>
		<SettingsRow
			icon="file"
			title="index.html"
			subtitle="записи и комментарии"
			chevron={false}
		/>

		<Label style="margin-top:18px">Раскладка</Label>
		<ChipGroup>
			<Chip selected={layout === 'feed'} onclick={() => (layout = 'feed')}>Единая лента</Chip>
			<Chip selected={layout === 'posts'} onclick={() => (layout = 'posts')}>
				По файлу на пост
			</Chip>
		</ChipGroup>
		<Hint
			>Единая лента — один файл, круг целиком в том порядке, в котором вы его читали. «По файлу на
			пост» — оглавление и страница на каждую запись. <span class="mono">index.html</span> есть в
			обоих случаях.</Hint
		>
		<Hint>Открывается офлайн и без Wynd: стили внутри файла, пути относительные, цвет круга на месте.</Hint>

		<Label style="margin-top:18px">Границы</Label>
		<SettingsRow
			title="Снято на {formatEntryDate(cycle.cutoff_date)}"
			value={cycle.cutoff_locked ? 'дата замерла' : 'можно сдвинуть'}
			chevron={false}
			style="border-top:1px solid var(--line)"
		/>
		<SettingsRow
			title="Скачать до {formatEntryDate(cycle.deadline.slice(0, 10))}"
			value="осталось {daysUntil(cycle.deadline)} дней"
			chevron={false}
		/>
		<SettingsRow title="Ваша видимость" value="то, что вы застали" chevron={false} />

		<Button variant="colored" style="margin-top:16px" onclick={download}>
			Скачать {formatBytes(cycle.personal_archive_bytes)}
		</Button>
		<Hint style="text-align:center;margin-top:12px"
			>Ссылка не одноразовая: качайте сколько нужно до {formatDeadline(cycle.deadline)}.</Hint
		>
	{/if}
</FormLayout>
