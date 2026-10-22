<script lang="ts">
	import { goUp } from '$lib/navigation/up';
	import { goto } from '$app/navigation';
	import { getContext, onMount } from 'svelte';
	import Button from '$ui/forms/Button.svelte';
	import DangerZone from '$ui/forms/DangerZone.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Input from '$ui/forms/Input.svelte';
	import Label from '$ui/forms/Label.svelte';
	import Loading from '$ui/Loading.svelte';
	import Switch from '$ui/forms/Switch.svelte';
	import TextButton from '$ui/forms/TextButton.svelte';
	import SettingsRow from '$ui/data/SettingsRow.svelte';
	import StatusCard from '$ui/data/StatusCard.svelte';
	import ConfirmDialog from '$ui/overlays/ConfirmDialog.svelte';
	import FormLayout from '$lib/layouts/FormLayout.svelte';
	import { authErrorHint } from '$lib/auth/auth';
	import { ApiError, isAccessError } from '$lib/api/client';
	import { deriveArchiveKey } from '$lib/archive/fullkey';
	import { suggestArchivePassword } from '$lib/archive/password';
	import {
		downloadFullArchive,
		fetchFullArchive,
		removeFrozenCircle,
		removeFullArchive,
		reviveCircle,
		startFullArchive,
		streamFullArchive,
		type FullArchiveState
	} from '$lib/circles/fullarchive';
	import { formatBytes } from '$lib/format/bytes';
	import { WORD, plural } from '$lib/format/plural';
	import { formatClock, formatSessionDay } from '$lib/format/time';
	import { CIRCLE_CTX, type CircleContext } from '$lib/journal/context';

	// Полный архив круга (план 51). Без удаления круга: собрать (6.24),
	// собирается (6.26), готов (6.27). С удалением: собрать (6.25), собирается
	// (6.26), файл не скачан (6.28), скачан (6.29), срок вышел (6.30). Готовый
	// файл положить некуда — скачать сразу (6.31).
	const circle = getContext<CircleContext>(CIRCLE_CTX);

	let info = $state<FullArchiveState | undefined>();
	let error = $state('');
	let loading = $state(true);
	let busy = $state(false);
	let busyNote = $state('');
	let encrypt = $state(true);
	let password = $state(suggestArchivePassword());
	let deleteCircle = $state(false);
	// «Собрать архив ещё раз» (6.29, 6.30): снова форма, пароль задаётся заново.
	let rebuild = $state(false);
	let askRemove = $state(false);

	const archive = $derived(info?.archive ?? null);
	const frozenAt = $derived(info?.frozen_at ?? '');
	const view = $derived.by(() => {
		const a = archive;
		if (rebuild || !a) return 'form';
		if (a.status === 'building') return 'building';
		if (a.status === 'ready') return a.delete_circle ? 'notDownloaded' : 'ready';
		if (a.status === 'removed' && a.delete_circle && frozenAt) {
			if (a.removed_reason === 'downloaded') return 'downloaded';
			if (a.removed_reason === 'expired' || a.removed_reason === 'admin') return 'gone';
		}
		return 'form';
	});

	function at(iso: string | undefined | null): string {
		return iso ? `${formatSessionDay(iso)} в ${formatClock(iso)}` : '';
	}

	async function load() {
		error = '';
		try {
			info = await fetchFullArchive(circle.origin, circle.circleId);
			// Круг уже замер — новый архив только с удалением.
			if (info.frozen_at) deleteCircle = true;
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

	async function act(fn: () => Promise<unknown>) {
		busy = true;
		error = '';
		try {
			await fn();
		} catch (err) {
			if (err instanceof ApiError && err.code === 'no_room') {
				// Место кончилось, пока экран был открыт, — путь 6.31.
				await load();
			} else {
				error = authErrorHint(err);
			}
		} finally {
			busy = false;
			busyNote = '';
		}
	}

	const build = () =>
		act(async () => {
			await startFullArchive(circle.origin, circle.circleId, await key(), deleteCircle);
			rebuild = false;
			password = suggestArchivePassword();
			await load();
			await circle.refresh();
		});

	const streamNow = () =>
		act(async () => {
			await streamFullArchive(circle.origin, circle.circleId, await key(), deleteCircle);
			if (deleteCircle) await circle.refresh();
		});

	// Скачивание идёт в браузере мимо страницы: что файл скачан, знает только
	// сервер. Пока ждём — спрашиваем его сами, иначе «Убрать круг с сервера»
	// появлялось лишь после ручного обновления (владелец, 2026-10-22).
	let downloadStarted = $state(false);
	const download = () =>
		act(async () => {
			await downloadFullArchive(circle.origin, circle.circleId);
			downloadStarted = true;
		});

	const waiting = $derived(view === 'building' || (downloadStarted && view === 'notDownloaded'));
	$effect(() => {
		if (!waiting) return;
		const timer = setInterval(() => {
			if (!busy) void load();
		}, 3000);
		return () => clearInterval(timer);
	});
	$effect(() => {
		const onVisible = () => {
			if (document.visibilityState === 'visible') void load();
		};
		document.addEventListener('visibilitychange', onVisible);
		return () => document.removeEventListener('visibilitychange', onVisible);
	});

	const remove = () =>
		act(async () => {
			await removeFullArchive(circle.origin, circle.circleId);
			password = suggestArchivePassword();
			await load();
		});

	const revive = () =>
		act(async () => {
			await reviveCircle(circle.origin, circle.circleId);
			rebuild = false;
			deleteCircle = false;
			password = suggestArchivePassword();
			await load();
			await circle.refresh();
		});

	const removeCircle = () =>
		act(async () => {
			await removeFrozenCircle(circle.origin, circle.circleId);
			askRemove = false;
			await goto('/circles');
		});

	onMount(() => void load());
</script>

{#snippet fileRow(withDownload: boolean)}
	{#if archive}
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
			trailingIcon={withDownload ? 'download' : undefined}
			chevron={false}
			divided="both"
			onclick={withDownload ? () => void download() : undefined}
		/>
	{/if}
{/snippet}

{#snippet reviveLink()}
	<Hint class="mt-14 ctr-text">
		<TextButton disabled={busy} onclick={() => void revive()}>Отменить — оживить круг</TextButton>
	</Hint>
{/snippet}

{#snippet removeLocked()}
	<!-- 6.28, 6.30: убрать круг, не скачав файл, нельзя. -->
	<DangerZone class="mt-20" items={['Убрать круг с сервера']} disabled>
		Сначала скачайте файл: без него круг не вернуть.
	</DangerZone>
{/snippet}

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
	{:else if view === 'building' && archive}
		<!-- 6.26: сборка идёт на сервере; о готовности экран узнаёт при открытии. -->
		<StatusCard
			title="Сервер собирает архив"
			text="Когда файл будет готов, придёт письмо."
		/>
		<Hint class="mt-14">
			<!-- С удалением круг замер в момент снимка — одна строка вместо двух с тем же временем. -->
			{#if archive.delete_circle && frozenAt}Круг замер {at(frozenAt)}.{:else}В файле будет круг,
				каким он был {at(archive.snapshot_at ?? archive.requested_at)}. Круг работает как обычно.{/if}
		</Hint>
		{#if archive.delete_circle}
			{@render reviveLink()}
		{:else}
			<Hint class="mt-22 ctr-text">
				<TextButton disabled={busy} onclick={() => void remove()}>Отменить сборку</TextButton>
			</Hint>
		{/if}
	{:else if view === 'ready' && archive}
		<!-- 6.27: файл готов, круг остаётся. -->
		{@render fileRow(true)}
		<Hint>
			В файле круг, каким он был {at(archive.snapshot_at ?? archive.ready_at)}. Круг работает как
			обычно.
		</Hint>
		<Hint class="mt-10 ink">
			Сервер удалит файл, когда вы его скачаете, или {at(archive.expires_at)}, если не скачаете.
		</Hint>
		<Hint class="mt-22 ctr-text">
			<TextButton disabled={busy} onclick={() => void remove()}>Удалить архив, не скачивая</TextButton>
		</Hint>
	{:else if view === 'notDownloaded' && archive}
		<!-- 6.28: файл готов и ещё не скачан, круг замер. -->
		{@render fileRow(true)}
		<Hint>Круг замер {at(frozenAt)}.</Hint>
		<Hint class="mt-10 ink">
			Сервер удалит файл, когда вы его скачаете, или {at(archive.expires_at)}, если не скачаете.
		</Hint>
		{@render removeLocked()}
		{@render reviveLink()}
	{:else if view === 'downloaded' && archive}
		<!-- 6.29: файл у владельца, на сервере его нет; круг можно убрать. -->
		{@render fileRow(false)}
		<Hint>Круг замер {at(frozenAt)}.</Hint>
		<Hint class="mt-10 ink">
			Вы скачали файл {at(archive.downloaded_at)}. На сервере его больше нет.
		</Hint>
		<Hint class="mt-10">
			<TextButton disabled={busy} onclick={() => (rebuild = true)}>Собрать архив ещё раз</TextButton>
		</Hint>
		<DangerZone class="mt-20" items={['Убрать круг с сервера']} onitem={() => (askRemove = true)}>
			Останется только файл у вас: потеряете файл или пароль — круг не вернуть.<br /><br />Участники получили письмо {at(frozenAt)}: свой архив они могут скачать,
			пока круг здесь.
		</DangerZone>
		{@render reviveLink()}
	{:else if view === 'gone' && archive}
		<!-- 6.30: срок вышел (или файл стёр администратор), круг стоит замершим. -->
		<StatusCard
			title="Файла на сервере нет"
			text={archive.removed_reason === 'admin'
				? `Архив был собран ${at(archive.ready_at)}. Вы его не скачали, и ${at(archive.removed_at)} администратор сервера удалил файл.`
				: `Архив был собран ${at(archive.ready_at)}. Вы его не скачали, и ${at(archive.removed_at)} сервер удалил файл.`}
		/>
		<Hint class="mt-14">Круг замер {at(frozenAt)}.</Hint>
		<Button class="mt-14" variant="colored" onclick={() => (rebuild = true)}>
			Собрать архив ещё раз
		</Button>
		{@render removeLocked()}
		{@render reviveLink()}
	{:else}
		<!-- 6.24 / 6.25 и, когда готовый файл положить некуда, 6.31. -->
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
		{/if}
		<SettingsRow
			class={encrypt ? 'mt-10' : ''}
			title="Удалить круг с сервера"
			subtitle="после того как файл скачан"
			divided="both"
		>
			{#snippet control()}
				<Switch
					bind:checked={deleteCircle}
					disabled={Boolean(frozenAt)}
					label="Удалить круг с сервера"
				/>
			{/snippet}
		</SettingsRow>
		{#if archive?.status === 'failed'}
			<Hint class="mt-10 ink">Прошлая сборка не удалась.</Hint>
		{/if}
		{#if info.fits}
			{#if deleteCircle}
				<!-- Без удаления круга шаги очевидны — их нет (C44). -->
				<Label class="mt-16">Как это пройдёт</Label>
				<Hint class="ink">
					1. Круг замирает: читать можно, писать нельзя. Участникам уходит письмо.<br />2. Вы
					скачиваете файл. Участники, пока круг здесь, могут скачать свой архив.<br />3. Вы
					убираете круг с сервера. Остаётся только файл.<br />4. Круг можно оживить из файла — здесь
					или на другом сервере.
				</Hint>
			{/if}
			<Button
				class={deleteCircle ? 'mt-14' : 'mt-22'}
				variant="colored"
				loading={busy}
				disabled={encrypt && !password.trim()}
				onclick={() => void build()}
			>
				{deleteCircle && !frozenAt ? 'Остановить круг и собрать архив' : 'Собрать архив'}
			</Button>
		{:else}
			<StatusCard
				title="Готовый файл положить некуда"
				text={`Архиву нужно ${formatBytes(info.estimate_bytes)}, а на сервере${circle.serverName ? ` «${circle.serverName}»` : ''} столько места нет. Скачайте его сразу: если скачивание оборвётся, оно начнётся заново.`}
			/>
			<Button
				class="mt-14"
				variant="colored"
				loading={busy}
				disabled={encrypt && !password.trim()}
				onclick={() => void streamNow()}
			>
				Скачать сразу
			</Button>
		{/if}
		{#if busyNote}
			<Hint class="mt-8">{busyNote}</Hint>
		{/if}
		{#if frozenAt}
			{@render reviveLink()}
		{/if}
	{/if}
	{#if error && info}
		<Hint class="mt-12">{error}</Hint>
	{/if}

	{#if askRemove}
		<ConfirmDialog
			title="Убрать круг с сервера?"
			confirmLabel="Убрать"
			cancelLabel="Оставить"
			loading={busy}
			onconfirm={() => void removeCircle()}
			oncancel={() => (askRemove = false)}
		>
			<Hint>Круг «{circle.name}» останется только в файле у вас.</Hint>
		</ConfirmDialog>
	{/if}
</FormLayout>
