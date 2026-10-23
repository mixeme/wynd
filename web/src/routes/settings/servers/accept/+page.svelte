<script lang="ts">
	import { goUp } from '$lib/navigation/up';
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import { onMount } from 'svelte';
	import Button from '$ui/forms/Button.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Input from '$ui/forms/Input.svelte';
	import SizeField from '$ui/forms/SizeField.svelte';
	import Label from '$ui/forms/Label.svelte';
	import Meter from '$ui/forms/Meter.svelte';
	import Switch from '$ui/forms/Switch.svelte';
	import FilePicker from '$ui/forms/FilePicker.svelte';
	import TextButton from '$ui/forms/TextButton.svelte';
	import CircleRow from '$ui/data/CircleRow.svelte';
	import ServerRow from '$ui/data/ServerRow.svelte';
	import SettingsRow from '$ui/data/SettingsRow.svelte';
	import StatusCard from '$ui/data/StatusCard.svelte';
	import FormLayout from '$lib/layouts/FormLayout.svelte';
	import { loadSourceUrl, sourceUrl } from '$lib/instance/source.svelte';
	import { authErrorHint } from '$lib/auth/auth';
	import { displayHost } from '$lib/auth/origin';
	import { circleInitial } from '$lib/circles/meta';
	import { CIRCLE_COLORS, type CircleColor } from '$lib/theme/colors';
	import { formatBytes } from '$lib/format/bytes';
	import { WORD, plural } from '$lib/format/plural';
	import { formatAdminDayYear, formatEntryDate, formatSessionDay } from '$lib/format/time';
	import { loadSessions } from '$lib/session/session.svelte';
	import { WrongPasswordError } from '$lib/archive/wyndfile';
	import {
		fetchInvitable,
		fetchReviveRoom,
		noRoomText,
		roomCaption,
		loadArchive,
		openFile,
		requestRevivePlace,
		reviveCircle,
		unlock,
		uploadArchiveFiles,
		type LoadedArchive,
		type OpenedFile,
		type ReviveResult,
		type ReviveRoom
	} from '$lib/archive/revive';

	// «Принять круг» (7.19) и оживление из архива (7.27–7.32, 7.33, 7.34).
	// Файл и пароль живут только на этом экране: уйти — значит выбрать файл
	// заново (7.31).
	// Просим столько, сколько нужно кругу, с запасом до мегабайта: при
	// округлении до гигабайта «попросить меньше» после отказа было нельзя.
	const MB = 1024 * 1024;
	// У сервера на своём адресе origin — пустая строка: не проверять как истину.
	const origin = $page.url.searchParams.get('origin') ?? '';
	// Ссылка «запустите свой» ведёт на исходники этого сервера (LIC-2).
	$effect(() => {
		loadSourceUrl(origin);
	});

	type Step = 'choose' | 'password' | 'review' | 'upload' | 'done';
	let step = $state<Step>('choose');
	let serverName = $state('');
	let host = $state(displayHost(origin));
	let freeRoom = $state<ReviveRoom | undefined>();
	let file = $state<File | undefined>();
	let opened = $state<OpenedFile | undefined>();
	let password = $state('');
	let passwordWrong = $state(false);
	let archive = $state<LoadedArchive | undefined>();
	let room = $state<ReviveRoom | undefined>();
	let sendInvites = $state(false);
	let sent = $state(0);
	let total = $state(0);
	let result = $state<ReviveResult | undefined>();
	let busy = $state(false);
	let busyNote = $state('');
	let error = $state('');
	let picker = $state<FilePicker | undefined>();

	const need = $derived(archive?.circleBytes ?? 0);
	// Итог оживления (7.33) переживает уход в участники и шаг назад: архива в
	// памяти тогда уже нет, имя и цвет круга — из sessionStorage (владелец,
	// 2026-10-22: «Провалился в неё, а вернулся в настройки круга»).
	let doneCircle = $state<{ name: string; color: string } | undefined>();
	const circleName = $derived(String(archive?.manifest.circle_name ?? doneCircle?.name ?? ''));
	// Экран оживления — цветом самого круга, как 7.29: плитка и «Оживить круг».
	// Без него акцент — цвет текста, и в тёмной теме кнопка была светлой с
	// белой надписью (владелец, 2026-10-22: «Кнопки странного цвета»).
	const circleColor = $derived(
		(archive?.circleColor ?? doneCircle?.color ?? '') in CIRCLE_COLORS
			? ((archive?.circleColor ?? doneCircle?.color) as CircleColor)
			: undefined
	);
	// Скольким уйдёт письмо — спрашиваем сервер: кто уже здесь, тому писать не
	// нужно. Ноль — переключателя нет (владелец, 2026-10-23: «Если письма не
	// нужны, то надо было переключатель убрать, чтобы не путать людей»).
	let invitable = $state(0);

	onMount(async () => {
		restoreDone();
		const sessions = await loadSessions();
		const s = sessions.find((x) => x.origin === origin);
		serverName = s?.name ?? '';
		try {
			freeRoom = await fetchReviveRoom(origin, 0);
		} catch {
			/* без «свободно» */
		}
	});

	async function pick(files: File[]) {
		const f = files[0];
		if (!f) return;
		error = '';
		file = f;
		// Большой файл читается не сразу — экран не молчит (владелец, 2026-10-23:
		// «после выбора файла архива на экране долго ничего не происходит»).
		// Шаг назван строкой: с телефона видно, что тормозит — файл или сервер.
		readingNote = 'Читаем файл…';
		try {
			opened = await openFile(f);
			if (opened.encrypted) {
				step = 'password';
			} else {
				await load(opened.src, (note) => (readingNote = note));
			}
		} catch (err) {
			error = 'Это не файл архива круга Wynd.';
			console.warn('revive: open', err);
		} finally {
			readingNote = '';
		}
	}

	async function openEncrypted() {
		if (!opened || !password.trim()) return;
		busy = true;
		passwordWrong = false;
		busyNote = 'Открываем файл…';
		try {
			await load(await unlock(opened, password.trim()), (note) => (busyNote = note));
		} catch (err) {
			if (err instanceof WrongPasswordError) passwordWrong = true;
			else error = 'Файл не открылся: он повреждён или это не архив круга Wynd.';
		} finally {
			busy = false;
			busyNote = '';
		}
	}

	async function load(src: Parameters<typeof loadArchive>[0], setNote: (note: string) => void) {
		archive = await loadArchive(src);
		placeBytes = Math.ceil(archive.circleBytes / MB) * MB;
		setNote('Спрашиваем сервер о месте…');
		// Сбой связи — не «это не файл архива»: файл уже прочитан.
		try {
			room = await fetchReviveRoom(origin, archive.circleBytes);
		} catch (err) {
			error = authErrorHint(err);
			return;
		}
		invitable = await fetchInvitable(origin, archive.emails).catch(() => Math.max(0, archive!.emails.length - 1));
		if (invitable === 0) sendInvites = false;
		step = 'review';
	}

	let placeBytes = $state(0);
	// Все, кроме оживившего, уже на этом сервере: писать некому.
	const allHere = $derived(
		!!result && result.invited === 0 && result.not_joined === 0 && (result.already_here ?? 0) > 0
	);
	let readingNote = $state('');

	async function askPlace() {
		if (!archive) return;
		if (placeBytes < need) {
			error = `Попросите не меньше ${formatBytes(need)} — столько нужно кругу`;
			return;
		}
		busy = true;
		error = '';
		try {
			await requestRevivePlace(origin, placeBytes, circleName);
			room = await fetchReviveRoom(origin, archive.circleBytes);
		} catch (err) {
			error = authErrorHint(err);
		} finally {
			busy = false;
		}
	}

	async function revive() {
		if (!archive) return;
		error = '';
		step = 'upload';
		try {
			const map = await uploadArchiveFiles(origin, archive, (s, t) => {
				sent = s;
				total = t;
			});
			result = await reviveCircle(origin, archive, map, sendInvites);
			step = 'done';
			keepDone();
		} catch (err) {
			error = authErrorHint(err);
			step = 'review';
		}
	}

	const DONE_KEY = 'wynd:revive-done';

	function keepDone() {
		if (!result) return;
		try {
			sessionStorage.setItem(
				DONE_KEY,
				JSON.stringify({ result, name: circleName, color: archive?.circleColor ?? '' })
			);
		} catch {
			/* без хранилища итог живёт, пока открыт экран */
		}
		void goto(doneUrl(result.circle_id), { replaceState: true, noScroll: true, keepFocus: true });
	}

	// Адрес итога: тот же экран с ?done=, сервер (?origin=) не теряется.
	function doneUrl(circleId: string): string {
		const here = new URL($page.url);
		here.searchParams.set('done', circleId);
		return here.pathname + here.search;
	}

	function restoreDone() {
		const id = $page.url.searchParams.get('done');
		if (!id) return;
		try {
			const saved = JSON.parse(sessionStorage.getItem(DONE_KEY) ?? 'null');
			if (saved?.result?.circle_id !== id) return;
			result = saved.result;
			doneCircle = { name: String(saved.name ?? ''), color: String(saved.color ?? '') };
			step = 'done';
		} catch {
			/* нечего вернуть — экран с начала */
		}
	}

	function day(iso: string | undefined): string {
		return iso ? formatSessionDay(iso) : '';
	}
</script>

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
	shell={!circleColor}
	color={circleColor}
	app
	title={step === 'done' ? circleName : step === 'choose' ? 'Принять круг' : 'Оживить круг'}
	onback={() => goUp('/settings/servers')}
>
	<FilePicker bind:this={picker} accept=".wynd" onfiles={(files) => void pick(files)} />
	{#if step === 'choose'}
		<!-- 7.19: две дороги — перенос по ссылке (план 53) и оживление из файла. -->
		<ServerRow
			name={serverName || host}
			subtitle={freeRoom ? `${host} · ${roomCaption(freeRoom, formatBytes)}` : host}
			card
			top={10}
		/>
		<SettingsRow
			class="mt-14"
			title="Перенести с другого сервера"
			subtitle="по ссылке переноса — сервер скачает круг сам"
			divided="top"
			onclick={() => goto(`/settings/servers/accept/move?origin=${encodeURIComponent(origin)}`)}
		/>
		<SettingsRow
			title="Оживить из архива"
			subtitle="из файла круга на этом устройстве"
			divided="both"
			onclick={() => picker?.open()}
		/>
		<Hint>Круг будет перенесён полностью. Вы станете его владельцем.</Hint>
		{#if readingNote}
			<Hint class="ink">{readingNote}</Hint>
		{/if}
	{:else if step === 'password' && file}
		<!-- 7.27 / 7.28: файл расшифровывается на этом устройстве. -->
		<Label>Файл архива</Label>
		<SettingsRow
			icon="file"
			title={file.name}
			subtitle={`${formatBytes(file.size)} · зашифрован`}
			chevron={false}
			divided="both"
		/>
		<Label class="mt-16">Пароль от файла</Label>
		<Input active bind:value={password} autocomplete="off" spellcheck="false" />
		{#if passwordWrong}
			<Hint class="ink">Пароль неверный</Hint>
		{:else}
			<Hint>
				Тот, что задали, когда собирали архив. Файл расшифровывается на этом устройстве, пароль на
				сервер не уходит.
			</Hint>
		{/if}
		<Button
			class="mt-14"
			variant="colored"
			loading={busy}
			disabled={!password.trim()}
			onclick={() => void openEncrypted()}
		>
			Открыть файл
		</Button>
		{#if busyNote}
			<Hint class="mt-8">{busyNote}</Hint>
		{/if}
	{:else if step === 'review' && archive && room}
		<!-- 7.29 и, когда круг не помещается, 7.30 / 7.31 / 7.32. -->
		<Label>Сервер</Label>
		<ServerRow
			name={serverName || host}
			subtitle={room.fits
				? `${host} · ${roomCaption(room, formatBytes)}`
				: `${host} · ${roomCaption(room, formatBytes)}`}
			card
			top={4}
		/>
		<Hint>
			Сервер хранит данные незашифрованными. Выбирайте сервер, которому доверяете, или
			<a class="under" href={sourceUrl(origin)}>запустите свой</a>.
		</Hint>
		<Label class="mt-16">Архив</Label>
		<CircleRow
			initial={circleInitial(circleName)}
			name={circleName}
			preview={`${plural(archive.posts, WORD.post)} · ${plural(archive.members, WORD.member)} · ${formatBytes(archive.circleBytes)}`}
			time=""
		/>
		<Hint>
			Архив собран {formatAdminDayYear(archive.manifest.snapshot_at)} на сервере {displayHost(
				archive.manifest.server
			)}.{#if archive.lastEntry}{' '}Последняя запись — {formatEntryDate(archive.lastEntry)}.{/if}
		</Hint>
		{#if room.fits}
			{#if invitable === 0}
			<!-- Писать некому — переключатель виден, но погашен (владелец, 2026-10-23). -->
			<Label class="mt-16">Участники</Label>
			<SettingsRow
				title="Разослать приглашения"
				subtitle={archive.emails.length > 1
					? 'все участники уже на этом сервере'
					: 'кроме вас, в круге никого нет'}
				divided="both"
			>
				{#snippet control()}
					<Switch checked={false} disabled label="Разослать приглашения" />
				{/snippet}
			</SettingsRow>
			{:else}
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
				Почты в архиве — на день, когда его собрали. Кто войдёт сюда той же почтой, попадёт в своё
				имя.
			</Hint>
			{/if}
			<Button class="mt-14" variant="colored" onclick={() => void revive()}>
				Оживить круг
			</Button>
		{:else if room.request?.status === 'pending'}
			<StatusCard
				title="Место попросили"
				text={`${day(room.request.created_at)} вы попросили у администратора ${formatBytes(room.request.requested_bytes)} для этого круга. Ответ администратора придёт письмом. Когда место дадут, здесь появится кнопка «Оживить круг».`}
			/>
			<Hint class="mt-14">
				Можно уйти и вернуться позже: просьба останется, файл нужно будет выбрать заново.
			</Hint>
		{:else if room.request?.status === 'rejected'}
			<StatusCard
				title="В месте отказали"
				text={`${day(room.request.resolved_at ?? room.request.created_at)} администратор отказал в ${formatBytes(room.request.requested_bytes)} для этого круга. Можно попросить меньше — кругу нужно ${formatBytes(need)} — или выбрать другой сервер.`}
			/>
			{@render placeAsk('Попросить ещё раз')}
		{:else}
			<StatusCard
				title="Круг сюда не помещается"
				text={noRoomText(formatBytes(need), room, formatBytes)}
			/>
			{@render placeAsk('Попросить место')}
		{/if}
	{:else if step === 'upload'}
		<Label>Загружаем круг</Label>
		<Meter value={total ? sent / total : 0} max={1} />
		<Hint class="mt-8">
			{formatBytes(sent)} из {formatBytes(total)}. Не закрывайте страницу, пока круг загружается.
		</Hint>
	{:else if step === 'done' && result}
		<!-- 7.33 / 7.34. -->
		<StatusCard
			title="Круг на месте"
			text={result.verified
				? `${plural(result.posts, WORD.post)}, ${plural(result.names, WORD.name)}, ${formatBytes(result.bytes)} — сверено с файлом архива, ничего не потерялось.`
				: `${plural(result.posts, WORD.post)}, ${plural(result.names, WORD.name)}, ${formatBytes(result.bytes)} — с файлом архива сошлось не всё.`}
		/>
		{#if allHere}
			<!-- Все, кроме вас, уже на этом сервере: письма не нужны, круг у них в списке. -->
			<SettingsRow
				class="mt-14"
				title="Все уже на этом сервере"
				subtitle="круг уже в их списке кругов — письма не нужны"
				chevron={false}
				divided="both"
			/>
		{:else if result.invited > 0}
			<SettingsRow
				class="mt-14"
				title="Приглашения отправлены"
				value={String(result.invited)}
				chevron={false}
				divided="both"
			/>
		{:else}
			<SettingsRow
				class="mt-14"
				title="Приглашения не отправлены"
				subtitle="позвать можно сейчас или позже — в участниках круга"
				divided="both"
				onclick={() =>
					goto(
						`/circles/${result?.circle_id}/settings/members?back=${encodeURIComponent(doneUrl(result?.circle_id ?? ''))}`
					)}
			/>
		{/if}
		{#if !allHere}
			<Hint>Кто войдёт сюда той же почтой, попадёт в своё имя.</Hint>
		{/if}
		<Button class="mt-14" variant="colored" onclick={() => goto(`/circles/${result?.circle_id}`)}>
			Открыть круг
		</Button>
	{/if}
	{#if error}
		<Hint class="mt-12">{error}</Hint>
	{/if}
</FormLayout>
