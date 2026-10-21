<script lang="ts">
	import { goUp } from '$lib/navigation/up';
	import { goto } from '$app/navigation';
	import { getContext, onDestroy, onMount } from 'svelte';
	import Button from '$ui/forms/Button.svelte';
	import DangerZone from '$ui/forms/DangerZone.svelte';
	import FieldDisplay from '$ui/forms/FieldDisplay.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Label from '$ui/forms/Label.svelte';
	import Loading from '$ui/Loading.svelte';
	import TextButton from '$ui/forms/TextButton.svelte';
	import SettingsRow from '$ui/data/SettingsRow.svelte';
	import ConfirmDialog from '$ui/overlays/ConfirmDialog.svelte';
	import FormLayout from '$lib/layouts/FormLayout.svelte';
	import { authErrorHint } from '$lib/auth/auth';
	import { isAccessError } from '$lib/api/client';
	import { copyText } from '$lib/clipboard';
	import {
		cancelCircleMove,
		completeCircleMove,
		fetchCircleMove,
		serverHost,
		startCircleMove,
		type CircleMoveState
	} from '$lib/circles/move';
	import { getSession } from '$lib/idb/db';
	import { formatBytes } from '$lib/format/bytes';
	import { WORD, plural } from '$lib/format/plural';
	import { formatClock, formatSessionDay } from '$lib/format/time';
	import { CIRCLE_CTX, type CircleContext } from '$lib/journal/context';

	// Перенос круга на другой сервер (план 53), прежний сервер: что уедет и
	// как (6.33), ссылка и ход (6.34), новый сервер принял круг (6.35).
	const circle = getContext<CircleContext>(CIRCLE_CTX);

	let info = $state<CircleMoveState | undefined>();
	let error = $state('');
	let loading = $state(true);
	let busy = $state(false);
	let copied = $state(false);
	let askComplete = $state(false);
	let serverName = $state('');
	let timer: ReturnType<typeof setInterval> | undefined;

	const move = $derived(info?.move ?? null);
	const dest = $derived(serverHost(move?.dest_url));

	function at(iso: string | undefined | null): string {
		return iso ? `${formatSessionDay(iso)} в ${formatClock(iso)}` : '';
	}

	async function load() {
		try {
			info = await fetchCircleMove(circle.origin, circle.circleId);
			error = '';
		} catch (err) {
			error = isAccessError(err) ? 'Перенести круг может только его владелец' : authErrorHint(err);
		} finally {
			loading = false;
		}
	}

	// «Что происходит» живая (6.34): пока новый сервер забирает круг, экран
	// переспрашивает свой сервер.
	$effect(() => {
		const live = move && move.status !== 'accepted' && move.link_alive;
		if (live && !timer) {
			timer = setInterval(() => void load(), 5000);
		} else if (!live && timer) {
			clearInterval(timer);
			timer = undefined;
		}
	});

	async function act(fn: () => Promise<unknown>) {
		busy = true;
		error = '';
		try {
			await fn();
		} catch (err) {
			error = authErrorHint(err);
		} finally {
			busy = false;
		}
	}

	const start = () =>
		act(async () => {
			await startCircleMove(circle.origin, circle.circleId);
			await load();
			await circle.refresh();
		});

	const cancel = () =>
		act(async () => {
			await cancelCircleMove(circle.origin, circle.circleId);
			await load();
			await circle.refresh();
		});

	const complete = () =>
		act(async () => {
			await completeCircleMove(circle.origin, circle.circleId);
			askComplete = false;
			await goto('/circles');
		});

	async function copy() {
		if (!move?.link) return;
		await copyText(move.link);
		copied = true;
		setTimeout(() => (copied = false), 2000);
	}

	onMount(() => {
		void load();
		void getSession(circle.origin).then((s) => (serverName = s?.name ?? ''));
	});
	onDestroy(() => timer && clearInterval(timer));
</script>

<FormLayout
	app
	color={circle.color}
	title="Перенос круга"
	onback={() => goUp(`/circles/${circle.circleId}/settings`)}
>
	{#if loading}
		<Loading />
	{:else if !info}
		<Hint class="gutter">{error}</Hint>
	{:else if move?.status === 'accepted'}
		<!-- 6.35: новый сервер принял круг и сверил; удаляет круг владелец. -->
		<Label>Что происходит</Label>
		<SettingsRow
			icon="check"
			title={`Круг переехал на ${dest}`}
			subtitle={`${at(move.accepted_at)} · сверено: ${plural(move.accepted_posts ?? 0, WORD.post)}, ${plural(move.accepted_names ?? 0, WORD.name)}, ${formatBytes(move.accepted_bytes ?? 0)}`}
			chevron={false}
			divided="both"
		/>
		<Hint>
			Новый сервер подтвердил, что получил всё. Загляните туда и убедитесь сами: круг уже
			открывается.
		</Hint>
		<DangerZone class="mt-20" items={['Завершить перенос']} onitem={() => (askComplete = true)}>
			Круг и все его файлы удалятся с «{serverName || 'этого сервера'}» и пропадут из списка у
			участников. Им уйдёт письмо: круг переехал на {dest}.
		</DangerZone>
		<Hint class="mt-14 ctr-text">
			<TextButton disabled={busy} onclick={() => void cancel()}>Отменить перенос</TextButton>
		</Hint>
		<Hint class="ctr-text">Круг здесь оживёт. Поднятый на новом сервере останется там отдельной копией.</Hint>
	{:else if move}
		<!-- 6.34: круг замер, ссылка выдана; ход — живой. -->
		{#if move.link_alive && move.link}
			<Label>Ссылка переноса</Label>
			<FieldDisplay mono value={move.link} />
			<Button class="mt-12" variant="ghost" onclick={() => void copy()}>
				{copied ? 'Скопировано' : 'Скопировать'}
			</Button>
			<Hint class="mt-10">
				Одноразовая, живёт 72 часа. Кто вставит её на новом сервере, станет там владельцем круга —
				никому её не передавайте.
			</Hint>
		{/if}
		<Label class="mt-16">Что происходит</Label>
		{#if !move.link_alive}
			<!-- Срок ссылки вышел, а новый сервер круг так и не принял (2.33 с этой стороны). -->
			<SettingsRow
				icon="clock"
				title="Срок ссылки вышел"
				subtitle={`ссылка действовала до ${at(move.expires_at)}`}
				chevron={false}
				divided="both"
			/>
			<Hint>
				Круг стоит замершим. Отмените перенос — круг оживёт; чтобы перенести его, начните заново:
				будет новая ссылка.
			</Hint>
		{:else if move.status === 'pulling'}
			<SettingsRow
				icon="download"
				title={`${dest} забирает круг`}
				subtitle={move.archive_status === 'ready'
					? `${formatBytes(move.pulled_bytes)} из ${formatBytes(move.archive_bytes)}`
					: 'сервер собирает круг для переноса'}
				chevron={false}
				divided="both"
			/>
			<Hint>Обрыв между серверами не страшен: новый сервер догрузит с места обрыва, пока жива ссылка.</Hint>
		{:else}
			<SettingsRow
				icon="clock"
				title="Ждём новый сервер"
				subtitle={`круг замер ${at(info.frozen_at)}`}
				chevron={false}
				divided="both"
			/>
			<Hint>На новом сервере: «Настройки» → «Серверы» → «Принять круг» → эта ссылка.</Hint>
		{/if}
		<Hint class="mt-22 ctr-text">
			<TextButton disabled={busy} onclick={() => void cancel()}>Отменить перенос</TextButton>
		</Hint>
	{:else}
		<!-- 6.33: что уедет и как это пройдёт. -->
		<Label>Что уедет</Label>
		<SettingsRow
			title={`Весь журнал круга «${circle.name}»`}
			subtitle={`${plural(info.estimate.posts, WORD.post)} с комментариями и реакциями, дни, события`}
			chevron={false}
		/>
		<SettingsRow
			title={formatBytes(info.estimate.bytes)}
			subtitle="фотографии, видео, звук и файлы"
			chevron={false}
			divided="top"
		/>
		<SettingsRow
			title={plural(info.estimate.members, WORD.member)}
			subtitle="имена, фото и почта каждого"
			chevron={false}
			divided="both"
		/>
		<Hint>По почте люди вернутся в свои имена на новом сервере.</Hint>
		<Label class="mt-16">Как это пройдёт</Label>
		<Hint class="ink">
			1. Круг замирает: читать можно, писать нельзя.<br />2. Вы получаете ссылку и вставляете её на
			новом сервере: «Настройки» → «Серверы» → «Принять круг». Сервер забирает круг сам.<br />3.
			Возвращаетесь сюда и завершаете перенос — здесь круг удаляется.
		</Hint>
		{#if info.frozen_at}
			<Hint class="mt-14 ink">Круг уже замер: его убирают с сервера в архив. Переносить его нельзя.</Hint>
		{:else}
			<Button class="mt-14" variant="colored" loading={busy} onclick={() => void start()}>
				Остановить круг и получить ссылку
			</Button>
			<Hint class="mt-10 ctr-text">Пока перенос не завершён, его можно отменить — круг оживёт.</Hint>
		{/if}
	{/if}
	{#if error && info}
		<Hint class="mt-12">{error}</Hint>
	{/if}

	{#if askComplete}
		<ConfirmDialog
			title="Завершить перенос?"
			confirmLabel="Завершить"
			cancelLabel="Не сейчас"
			loading={busy}
			onconfirm={() => void complete()}
			oncancel={() => (askComplete = false)}
		>
			<Hint>Круг «{circle.name}» удалится отсюда. Останется только на {dest}.</Hint>
		</ConfirmDialog>
	{/if}
</FormLayout>
