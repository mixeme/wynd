<script lang="ts">
	import { goUp } from '$lib/navigation/up';
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import { onDestroy, onMount } from 'svelte';
	import Button from '$ui/forms/Button.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Input from '$ui/forms/Input.svelte';
	import SizeField from '$ui/forms/SizeField.svelte';
	import Label from '$ui/forms/Label.svelte';
	import Meter from '$ui/forms/Meter.svelte';
	import Switch from '$ui/forms/Switch.svelte';
	import TextButton from '$ui/forms/TextButton.svelte';
	import CircleRow from '$ui/data/CircleRow.svelte';
	import ServerRow from '$ui/data/ServerRow.svelte';
	import SettingsRow from '$ui/data/SettingsRow.svelte';
	import StatusCard from '$ui/data/StatusCard.svelte';
	import FormLayout from '$lib/layouts/FormLayout.svelte';
	import { loadSourceUrl, sourceUrl } from '$lib/instance/source.svelte';
	import { ApiError } from '$lib/api/client';
	import { authErrorHint } from '$lib/auth/auth';
	import { displayHost } from '$lib/auth/origin';
	import { circleInitial } from '$lib/circles/meta';
	import {
		fetchReviveRoom,
		noRoomText,
		requestRevivePlace,
		roomCaption,
		type ReviveRoom
	} from '$lib/archive/revive';
	import {
		fetchIncomingMove,
		moveErrorText,
		putIncomingMove,
		startIncomingMove,
		type IncomingMoveView
	} from '$lib/circles/movein';
	import { formatBytes } from '$lib/format/bytes';
	import { WORD, plural } from '$lib/format/plural';
	import { formatClock, formatSessionDay } from '$lib/format/time';
	import { loadSessions } from '$lib/session/session.svelte';
	import { CIRCLE_COLORS, type CircleColor } from '$lib/theme/colors';

	// «Перенести с другого сервера» (план 53): ссылка и карточка круга (7.20),
	// не помещается / место попросили / отказали (7.21, 7.22, 7.23), срок
	// ссылки вышел (7.24), ход, итог (7.25, 7.26). Ссылку сервер помнит вместе
	// с просьбой о месте: экран при открытии показывает её снова.
	// Просим столько, сколько нужно кругу, с запасом до мегабайта: при
	// округлении до гигабайта «попросить меньше» после отказа было нельзя.
	const MB = 1024 * 1024;
	// У сервера на своём адресе origin — пустая строка: не проверять как истину.
	const origin = $page.url.searchParams.get('origin') ?? '';
	// Ссылка «запустите свой» ведёт на исходники этого сервера (LIC-2).
	$effect(() => {
		loadSourceUrl(origin);
	});

	let serverName = $state('');
	const host = displayHost(origin);
	let freeRoom = $state<ReviveRoom | undefined>();
	let link = $state('');
	let move = $state<IncomingMoveView | undefined>();
	let sendInvites = $state(false);
	let busy = $state(false);
	let checking = $state(false);
	let error = $state('');
	// Итог показывается тому, кто поднимал круг с этого экрана, а не при
	// каждом открытии.
	let watching = $state(false);
	let timer: ReturnType<typeof setInterval> | undefined;

	const srv = $derived(serverName ? `«${serverName}»` : 'этого сервера');
	const source = $derived(move ? displayHost(move.source_url) : '');
	const sourceName = $derived(move?.source_name ? `«${move.source_name}»` : source);
	const active = $derived(move?.status === 'pulling' || move?.status === 'reviving');
	const expired = $derived(Boolean(move && !move.link_alive && move.status !== 'done'));

	function at(iso: string | undefined): string {
		return iso ? `${formatSessionDay(iso)} в ${formatClock(iso)}` : '';
	}
	function until(iso: string | undefined): string {
		return iso ? `${formatSessionDay(iso)}, ${formatClock(iso)}` : '';
	}
	function day(iso: string | undefined): string {
		return iso ? formatSessionDay(iso) : '';
	}
	function colorOf(token: string): string | undefined {
		return token in CIRCLE_COLORS ? CIRCLE_COLORS[token as CircleColor].cssVar : undefined;
	}

	async function refresh() {
		try {
			const res = await fetchIncomingMove(origin);
			const m = res.move;
			if (!m || (m.status === 'done' && !watching)) {
				move = undefined;
				if (res.room_bytes !== undefined) freeRoom = { ...res, room_bytes: res.room_bytes, fits: res.fits ?? true };
				return;
			}
			move = m;
			if (m.link && !link) link = m.link;
			if (m.status === 'draft') sendInvites = m.send_invites;
		} catch (err) {
			error = authErrorHint(err);
		}
	}

	// Ход живой: пока сервер забирает круг, экран переспрашивает его.
	$effect(() => {
		if (active && !timer) {
			timer = setInterval(() => void refresh(), 3000);
		} else if (!active && timer) {
			clearInterval(timer);
			timer = undefined;
		}
	});

	onMount(async () => {
		const sessions = await loadSessions();
		serverName = sessions.find((x) => x.origin === origin)?.name ?? '';
		await refresh();
		if (active) watching = true;
		if (freeRoom === undefined) {
			try {
				freeRoom = await fetchReviveRoom(origin, 0);
			} catch {
				/* без «свободно» */
			}
		}
	});
	onDestroy(() => timer && clearInterval(timer));

	async function check(value: string) {
		const text = value.trim();
		if (!text || text === move?.link) return;
		checking = true;
		error = '';
		try {
			move = await putIncomingMove(origin, text);
			sendInvites = false;
		} catch (err) {
			error = err instanceof ApiError ? moveErrorText(err.code) : authErrorHint(err);
		} finally {
			checking = false;
		}
	}

	function onPaste(event: ClipboardEvent) {
		const text = event.clipboardData?.getData('text') ?? '';
		if (!text.trim()) return;
		event.preventDefault();
		link = text.trim();
		void check(link);
	}

	let placeBytes = $state(0);
	// Сначала — вес круга с запасом до мегабайта; дальше человек меняет сам.
	$effect(() => {
		if (move && placeBytes === 0) placeBytes = Math.ceil(move.circle.bytes / MB) * MB;
	});

	async function askPlace() {
		if (!move) return;
		if (placeBytes < move.circle.bytes) {
			error = `Попросите не меньше ${formatBytes(move.circle.bytes)} — столько нужно кругу`;
			return;
		}
		busy = true;
		error = '';
		try {
			await requestRevivePlace(origin, placeBytes, move.circle.name);
			await refresh();
		} catch (err) {
			error = authErrorHint(err);
		} finally {
			busy = false;
		}
	}

	async function start() {
		if (!move) return;
		busy = true;
		error = '';
		try {
			await startIncomingMove(origin, move.id, sendInvites);
			watching = true;
			await refresh();
		} catch (err) {
			error = err instanceof ApiError ? moveErrorText(err.code) : authErrorHint(err);
			await refresh();
		} finally {
			busy = false;
		}
	}
</script>

{#snippet deadline()}
	Ссылка переноса действует до {until(move?.expires_at)}. До тех пор круг на прежнем сервере стоит
	замершим.
{/snippet}

{#snippet placeAsk(label: string)}
	<!-- Сколько попросить — человек выбирает сам (владелец, 2026-10-17): после
	     отказа можно попросить меньше. Сначала — вес круга с запасом до МБ. -->
	<Label class="mt-16">Сколько попросить</Label>
	<SizeField bind:bytes={placeBytes} />
	<Button class="mt-14" variant="colored" loading={busy} onclick={() => void askPlace()}>
		{label}
	</Button>
{/snippet}

<FormLayout
	shell
	app
	title={move?.status === 'done' ? move.circle.name : 'Перенос круга'}
	onback={() => goUp(`/settings/servers/accept?origin=${encodeURIComponent(origin)}`)}
>
	{#if move?.status === 'done' && move.result}
		<!-- 7.25 / 7.26: круг поднят и сверен; остаётся завершить перенос там. -->
		{@const r = move.result}
		<StatusCard
			title="Круг на месте"
			text={r.verified
				? `${plural(r.posts, WORD.post)}, ${plural(r.names, WORD.name)}, ${formatBytes(r.bytes)} — сверено с прежним сервером, ничего не потерялось.`
				: `${plural(r.posts, WORD.post)}, ${plural(r.names, WORD.name)}, ${formatBytes(r.bytes)} — с прежним сервером сошлось не всё.`}
		/>
		{#if r.invited > 0}
			<SettingsRow
				class="mt-14"
				title="Приглашения отправлены"
				value={String(r.invited)}
				chevron={false}
				divided="both"
			/>
		{:else}
			<SettingsRow
				class="mt-14"
				title="Приглашения не отправлены"
				subtitle="позвать можно сейчас или позже — в участниках круга"
				divided="both"
				onclick={() => goto(`/circles/${r.circle_id}/settings/members`)}
			/>
		{/if}
		<Hint>
			Осталось завершить перенос на прежнем сервере: {sourceName} → настройки круга → «Завершить
			перенос». До тех пор круг стоит там замершим.
		</Hint>
		<Button class="mt-14" variant="colored" onclick={() => goto(`/circles/${r.circle_id}`)}>
			Открыть круг
		</Button>
	{:else if move && active}
		<!-- Ход: сервер забирает и поднимает круг сам (кадра нет). -->
		<CircleRow
			initial={circleInitial(move.circle.name)}
			name={move.circle.name}
			color={colorOf(move.circle.color)}
			preview={`${plural(move.circle.posts, WORD.post)} · ${plural(move.circle.members, WORD.member)} · ${formatBytes(move.circle.bytes)}`}
			time=""
		/>
		<Label class="mt-16">Что происходит</Label>
		{#if move.status === 'reviving'}
			<Hint class="ink">Круг здесь. Сервер переносит его и сверяет с прежним.</Hint>
		{:else}
			<Meter value={move.circle.bytes ? Math.min(1, move.pulled_bytes / move.circle.bytes) : 0} max={1} />
			<Hint class="mt-8">
				{move.pulled_bytes
					? `Сервер скачивает круг с ${source}: ${formatBytes(move.pulled_bytes)} из ${formatBytes(move.circle.bytes)}.`
					: `${move.source_name ? `«${move.source_name}»` : source} собирает круг для переноса.`}
			</Hint>
		{/if}
		<Hint class="mt-14 ctr-text">Приложение можно закрыть: серверы работают между собой.</Hint>
	{:else}
		<!-- 7.20 и его состояния. -->
		<Label>Сервер</Label>
		<ServerRow
			name={serverName || host}
			subtitle={move && !move.fits
				? `${host} · ${roomCaption(move, formatBytes)}`
				: freeRoom !== undefined || move
					? `${host} · ${roomCaption(move ?? freeRoom ?? { room_bytes: 0 }, formatBytes)}`
					: host}
			card
			top={4}
		/>
		{#if !move || move.fits}
			<Hint>
				Сервер хранит данные незашифрованными. Выбирайте сервер, которому доверяете, или
				<a class="under" href={sourceUrl(origin)}>запустите свой</a>.
			</Hint>
		{/if}
		<Label class="mt-16">Ссылка переноса</Label>
		<Input
			bind:value={link}
			small
			gray={expired}
			placeholder="https://…/move/…"
			autocomplete="off"
			spellcheck="false"
			onpaste={onPaste}
			onchange={() => void check(link)}
		/>
		{#if checking}
			<Hint>Спрашиваем прежний сервер…</Hint>
		{:else if !move}
			<Hint>
				Ссылку получает владелец круга на прежнем сервере: настройки круга → «Перенести на другой
				сервер».
			</Hint>
		{/if}
		{#if move}
			<CircleRow
				class="mt-10"
				initial={circleInitial(move.circle.name)}
				name={move.circle.name}
				color={colorOf(move.circle.color)}
				preview={`${plural(move.circle.posts, WORD.post)} · ${plural(move.circle.members, WORD.member)} · ${formatBytes(move.circle.bytes)}`}
				time=""
			/>
			<!-- Прежний сервер — той же карточкой, что и этот (владелец, 2026-10-17). -->
			<Label class="mt-16">Прежний сервер</Label>
			<ServerRow name={move.source_name || source} subtitle={source} card top={4} />
			{#if move.frozen_at}
				<Hint>Круг замер {at(move.frozen_at)}.</Hint>
			{/if}

			{#if expired}
				<!-- 7.24: срок ссылки вышел. -->
				<StatusCard
					class="mt-14"
					title="Срок ссылки вышел"
					text={`Ссылка переноса действовала до ${until(move.expires_at)}. На прежнем сервере${move.source_name ? `, «${move.source_name}»,` : ''} отмените перенос и начните его заново — там дадут новую ссылку.${move.request?.status === 'pending' ? ` Просьба о ${formatBytes(move.request.requested_bytes)} у администратора ${srv} остаётся.` : ''}`}
				/>
				<Hint class="mt-14 ctr-text">Новую ссылку вставьте в это же поле.</Hint>
			{:else if move.status === 'failed'}
				<StatusCard class="mt-14" title="Перенос не удался" text={moveErrorText(move.error)} />
				<Button class="mt-14" variant="colored" loading={busy} onclick={() => void start()}>
					Попробовать ещё раз
				</Button>
			{:else if move.fits}
				{#if move.circle.members > 1}
					<Label class="mt-16">Участники</Label>
					<SettingsRow
						title="Разослать приглашения"
						subtitle="письмо на почту каждому участнику"
						divided="both"
					>
						{#snippet control()}
							<Switch bind:checked={sendInvites} label="Разослать приглашения" />
						{/snippet}
					</SettingsRow>
					<Hint>
						Кто войдёт на этот сервер своей прежней почтой, попадёт в своё имя и свои записи — с письмом
						или без.
					</Hint>
				{/if}
				<Button class="mt-14" variant="colored" loading={busy} onclick={() => void start()}>
					Перенести круг
				</Button>
				<Hint class="mt-8 ctr-text">Сервер скачает круг сам. Приложение можно закрыть.</Hint>
			{:else if move.request?.status === 'pending'}
				<!-- 7.22 -->
				<StatusCard
					class="mt-14"
					title="Место попросили"
					text={`${day(move.request.created_at)} вы попросили у администратора ${formatBytes(move.request.requested_bytes)} для этого круга. Ответ администратора придёт письмом. Когда место дадут, здесь появится кнопка «Перенести круг».`}
				/>
				<Hint class="mt-10">{@render deadline()}</Hint>
				<Hint class="mt-14 ctr-text">Можно уйти и вернуться позже: просьба и ссылка останутся.</Hint>
			{:else if move.request?.status === 'rejected'}
				<!-- 7.23 -->
				<StatusCard
					class="mt-14"
					title="В месте отказали"
					text={`${day(move.request.resolved_at ?? move.request.created_at)} администратор отказал в ${formatBytes(move.request.requested_bytes)} для этого круга. Можно попросить меньше — кругу нужно ${formatBytes(move.circle.bytes)} — или выбрать другой сервер.`}
				/>
				<Hint class="mt-10">{@render deadline()}</Hint>
				{@render placeAsk('Попросить ещё раз')}
			{:else}
				<!-- 7.21 -->
				<StatusCard
					class="mt-14"
					title="Круг сюда не помещается"
					text={noRoomText(formatBytes(move.circle.bytes), move, formatBytes)}
				/>
				<Hint class="mt-10">{@render deadline()}</Hint>
				{@render placeAsk('Попросить место')}
			{/if}
		{/if}
	{/if}
	{#if error}
		<Hint class="mt-12">{error}</Hint>
	{/if}
</FormLayout>
