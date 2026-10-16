<script lang="ts">
	import { goUp } from '$lib/navigation/up';
	import { goto } from '$app/navigation';
	import { getContext } from 'svelte';
	import Button from '$ui/forms/Button.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Label from '$ui/forms/Label.svelte';
	import SettingsRow from '$ui/data/SettingsRow.svelte';
	import FormLayout from '$lib/layouts/FormLayout.svelte';
	import { formatBytes } from '$lib/format/bytes';
	import { formatEntryDate, daysUntil, pluralFiles, pluralPosts } from '$lib/format/time';
	import { downloadArchive } from '$lib/journal/posts';
	import { CIRCLE_CTX, type CircleContext } from '$lib/journal/context';

	const circle = getContext<CircleContext>(CIRCLE_CTX);

	const cycle = $derived(circle.archiveCycle);

	function goBack() {
		goUp(`/circles/${circle.circleId}/settings`);
	}

	function download() {
		if (cycle?.download_url) {
			void downloadArchive(circle.origin, cycle.download_url);
		}
	}
</script>

<FormLayout app color={circle.color} title="Архив круга" onback={goBack}>
	{#if !cycle?.active}
		<Hint class="gutter">Сейчас архивация не идёт.</Hint>
	{:else}
		<Label>Что внутри</Label>
		<SettingsRow
			icon="photo"
			title="/media"
			subtitle="{pluralFiles(cycle.personal_archive_media_count)} · {formatBytes(
				cycle.personal_archive_bytes
			)}"
			chevron={false}
			divided="top"
		/>
		<SettingsRow
			icon="file"
			title="index.html"
			subtitle="единая лента: {pluralPosts(cycle.personal_archive_post_count)} и все комментарии к ним"
			chevron={false}
		/>
		<SettingsRow
			icon="file"
			title="/posts"
			subtitle="те же записи — по странице на каждую, с оглавлением"
			chevron={false}
		/>
		<Hint>Открывается офлайн и без Wynd: стили внутри файлов, пути относительные, цвет круга на месте.</Hint>

		<Label>Границы</Label>
		<SettingsRow
			title="Снято на {formatEntryDate(cycle.cutoff_date)}"
			value={cycle.cutoff_locked ? 'дата замерла' : 'можно сдвинуть'}
			chevron={false}
			divided="top"
		/>
		<SettingsRow
			title="Скачать до {formatEntryDate(cycle.deadline.slice(0, 10))}"
			value="осталось {daysUntil(cycle.deadline)} дней"
			chevron={false}
		/>
		<SettingsRow title="Ваша видимость" value="то, что вы застали" chevron={false} />

		<Button class="mt-16" variant="colored" onclick={download}>
			Скачать{cycle.personal_archive_bytes ? ` ${formatBytes(cycle.personal_archive_bytes)}` : ''}
		</Button>
		<!-- Архив идёт потоком (6.23, план 46, C38): докачки нет, повтор — заново. -->
		<Hint class="mt-12 ctr"
			>Скачивание начнётся сразу. Если оно оборвётся, начнётся заново. Качать можно сколько
			угодно раз до {formatEntryDate(cycle.deadline.slice(0, 10))}.</Hint
		>
	{/if}
</FormLayout>
