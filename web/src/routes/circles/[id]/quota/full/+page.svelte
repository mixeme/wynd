<script lang="ts">
	import { goUp } from '$lib/navigation/up';
	import { getContext, onMount } from 'svelte';
	import Button from '$ui/forms/Button.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Input from '$ui/forms/Input.svelte';
	import Label from '$ui/forms/Label.svelte';
	import Loading from '$ui/Loading.svelte';
	import Switch from '$ui/forms/Switch.svelte';
	import TextButton from '$ui/forms/TextButton.svelte';
	import SettingsRow from '$ui/data/SettingsRow.svelte';
	import StatusCard from '$ui/data/StatusCard.svelte';
	import FormLayout from '$lib/layouts/FormLayout.svelte';
	import { authErrorHint } from '$lib/auth/auth';
	import { ApiError, isAccessError } from '$lib/api/client';
	import { deriveArchiveKey } from '$lib/archive/fullkey';
	import { suggestArchivePassword } from '$lib/archive/password';
	import {
		downloadFullArchive,
		fetchFullArchive,
		removeFullArchive,
		startFullArchive,
		streamFullArchive,
		type FullArchiveState
	} from '$lib/circles/fullarchive';
	import { formatBytes } from '$lib/format/bytes';
	import { WORD, plural } from '$lib/format/plural';
	import { formatClock, formatSessionDay } from '$lib/format/time';
	import { CIRCLE_CTX, type CircleContext } from '$lib/journal/context';

	// Полный архив круга (план 51): собрать (6.29), собирается (6.40), готов
	// (6.33); готовый файл положить некуда — скачать сразу (6.43).
	const circle = getContext<CircleContext>(CIRCLE_CTX);

	let info = $state<FullArchiveState | undefined>();
	let error = $state('');
	let loading = $state(true);
	let busy = $state(false);
	let busyNote = $state('');
	let encrypt = $state(true);
	let password = $state(suggestArchivePassword());

	const archive = $derived(info?.archive ?? null);
	const building = $derived(archive?.status === 'building');
	const ready = $derived(archive?.status === 'ready');
	const failed = $derived(archive?.status === 'failed');

	function at(iso: string | undefined): string {
		return iso ? `${formatSessionDay(iso)} в ${formatClock(iso)}` : '';
	}

	async function load() {
		error = '';
		try {
			info = await fetchFullArchive(circle.origin, circle.circleId);
		} catch (err) {
			error = isAccessError(err)
				? 'Полный архив собирает только владелец круга'
				: authErrorHint(err);
		} finally {
			loading = false;
		}
	}

	async function key() {
		if (!encrypt) return undefined;
		busyNote = 'Готовим ключ из пароля…';
		return deriveArchiveKey(password.trim());
	}

	async function build() {
		if (encrypt && !password.trim()) return;
		busy = true;
		error = '';
		try {
			await startFullArchive(circle.origin, circle.circleId, await key());
			await load();
		} catch (err) {
			if (err instanceof ApiError && err.code === 'no_room') {
				// Место кончилось, пока экран был открыт, — путь 6.43.
				await load();
			} else {
				error = authErrorHint(err);
			}
		} finally {
			busy = false;
			busyNote = '';
		}
	}

	async function streamNow() {
		if (encrypt && !password.trim()) return;
		busy = true;
		error = '';
		try {
			await streamFullArchive(circle.origin, circle.circleId, await key());
		} catch (err) {
			error = authErrorHint(err);
		} finally {
			busy = false;
			busyNote = '';
		}
	}

	async function download() {
		error = '';
		try {
			await downloadFullArchive(circle.origin, circle.circleId);
		} catch (err) {
			error = authErrorHint(err);
		}
	}

	async function remove() {
		busy = true;
		error = '';
		try {
			await removeFullArchive(circle.origin, circle.circleId);
			password = suggestArchivePassword();
			await load();
		} catch (err) {
			error = authErrorHint(err);
		} finally {
			busy = false;
		}
	}

	onMount(() => void load());
</script>

<FormLayout
	app
	color={circle.color}
	title="Полный архив круга"
	onback={() => goUp(`/circles/${circle.circleId}/quota`)}
>
	{#if loading}
		<Loading />
	{:else if !info}
		<Hint class="gutter">{error}</Hint>
	{:else if building && archive}
		<!-- 6.40: сборка идёт на сервере; о готовности экран узнаёт при открытии. -->
		<StatusCard
			title="Сервер собирает архив"
			text={`Вы попросили собрать его ${at(archive.requested_at)}. Когда файл будет готов, придёт письмо — скачать его можно будет здесь.`}
		/>
		<Hint class="mt-14">
			В файле будет круг, каким он был {at(archive.snapshot_at ?? archive.requested_at)}. Написанное
			позже в него не попадёт.
		</Hint>
		<Hint class="mt-10">Круг работает как обычно.</Hint>
		<Hint class="mt-22 ctr-text">Можно уйти: сборка идёт на сервере.</Hint>
		<Hint class="mt-14 ctr-text">
			<TextButton disabled={busy} onclick={() => void remove()}>Отменить сборку</TextButton>
		</Hint>
	{:else if ready && archive}
		<!-- 6.33: файл готов, круг остаётся. -->
		<Label>Файл</Label>
		<SettingsRow
			icon="file"
			title={`${circle.name}.wynd`}
			subtitle={[
				formatBytes(archive.size_bytes),
				archive.encrypted ? 'зашифрован' : '',
				`собран ${at(archive.ready_at)}`
			]
				.filter(Boolean)
				.join(' · ')}
			trailingIcon="download"
			divided="both"
			onclick={() => void download()}
		/>
		<Hint>
			В файле круг, каким он был {at(archive.snapshot_at ?? archive.ready_at)}. Написанное позже в
			него не попадёт.
		</Hint>
		<Hint class="mt-10 ink">
			Сервер удалит файл, когда вы его скачаете, или {at(archive.expires_at)}, если не скачаете.
		</Hint>
		<Hint class="mt-10">Круг работает как обычно.</Hint>
		<Hint class="mt-22 ctr-text">
			<TextButton disabled={busy} onclick={() => void remove()}>Удалить архив, не скачивая</TextButton>
		</Hint>
	{:else}
		<!-- 6.29 и, когда готовый файл положить некуда, 6.43. -->
		<Label>Что уйдёт в файл</Label>
		<SettingsRow
			title={`Весь журнал «${circle.name}» и ${plural(info.members, WORD.member)}`}
			subtitle={`${plural(info.posts, WORD.post)}, имена, фото и почта каждого · ${formatBytes(info.estimate_bytes)}`}
			chevron={false}
		/>
		<SettingsRow
			title="Зашифровать файл"
			subtitle="пароль спросят, когда круг будут оживлять"
			divided="top"
		>
			{#snippet control()}
				<Switch bind:checked={encrypt} label="Зашифровать файл" />
			{/snippet}
		</SettingsRow>
		{#if encrypt}
			<Input active bind:value={password} autocomplete="off" spellcheck="false" />
			<Hint>Пароль нигде не хранится. Забудете — файл не открыть.</Hint>
		{:else}
			<Hint>Файл не зашифрован — храните его так же бережно, как сам сервер.</Hint>
		{/if}
		{#if failed}
			<Hint class="mt-10 ink">Прошлая сборка не удалась. Соберите архив ещё раз.</Hint>
		{/if}
		{#if info.fits}
			<Label class="mt-16">Как это пройдёт</Label>
			<Hint class="ink">
				1. Сервер собирает файл. Круг работает как обычно.<br />2. Вы скачиваете файл.<br />3.
				Круг остаётся на сервере.
			</Hint>
			<Button class="mt-14" variant="colored" loading={busy} disabled={encrypt && !password.trim()} onclick={() => void build()}>
				Собрать архив
			</Button>
		{:else}
			<StatusCard
				title="Готовый файл положить некуда"
				text={`Архиву нужно ${formatBytes(info.estimate_bytes)}, а на сервере${circle.serverName ? ` «${circle.serverName}»` : ''} столько места нет. Его можно скачать сразу: сервер соберёт файл прямо в скачивание. Если оно оборвётся, начнётся заново.`}
			/>
			<Button class="mt-14" variant="colored" loading={busy} disabled={encrypt && !password.trim()} onclick={() => void streamNow()}>
				Скачать сразу
			</Button>
		{/if}
		{#if busyNote}
			<Hint class="mt-8">{busyNote}</Hint>
		{/if}
	{/if}
	{#if error && info}
		<Hint class="mt-12">{error}</Hint>
	{/if}
</FormLayout>
