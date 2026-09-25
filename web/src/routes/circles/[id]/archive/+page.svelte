<script lang="ts">
	import { goto } from '$app/navigation';
	import { getContext } from 'svelte';
	import Button from '$ui/forms/Button.svelte';
	import Chip from '$ui/forms/Chip.svelte';
	import ChipGroup from '$ui/forms/ChipGroup.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Label from '$ui/forms/Label.svelte';
	import Icon from '$ui/Icon.svelte';
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
		<div class="row2" style="border-top:1px solid var(--line)">
			<Icon name="photo" />
			<div class="g">
				<div style="font-weight:600">/media</div>
				<div class="sub">фото и видео записей</div>
			</div>
		</div>
		<div class="row2">
			<Icon name="file" />
			<div class="g">
				<div style="font-weight:600">index.html</div>
				<div class="sub">записи и комментарии</div>
			</div>
		</div>

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
		<div class="row2" style="border-top:1px solid var(--line)">
			<div class="g">Снято на {formatEntryDate(cycle.cutoff_date)}</div>
			<span class="val">{cycle.cutoff_locked ? 'дата замерла' : 'можно сдвинуть'}</span>
		</div>
		<div class="row2">
			<div class="g">Скачать до {formatEntryDate(cycle.deadline.slice(0, 10))}</div>
			<span class="val">осталось {daysUntil(cycle.deadline)} дней</span>
		</div>
		<div class="row2">
			<div class="g">Ваша видимость</div>
			<span class="val">то, что вы застали</span>
		</div>

		<Button variant="colored" style="margin-top:16px" onclick={download}>
			Скачать {formatBytes(cycle.personal_archive_bytes)}
		</Button>
		<Hint style="text-align:center;margin-top:12px"
			>Ссылка не одноразовая: качайте сколько нужно до {formatDeadline(cycle.deadline)}.</Hint
		>
	{/if}
</FormLayout>
