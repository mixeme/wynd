<script lang="ts">
	import { goUp } from '$lib/navigation/up';
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import { onMount } from 'svelte';
	import Button from '$ui/forms/Button.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Input from '$ui/forms/Input.svelte';
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
	import { authErrorHint } from '$lib/auth/auth';
	import { displayHost } from '$lib/auth/origin';
	import { circleInitial } from '$lib/circles/meta';
	import { formatBytes } from '$lib/format/bytes';
	import { WORD, plural } from '$lib/format/plural';
	import { formatAdminDayYear, formatEntryDate, formatSessionDay } from '$lib/format/time';
	import { loadSessions } from '$lib/session/session.svelte';
	import { WrongPasswordError } from '$lib/archive/wyndfile';
	import {
		fetchReviveRoom,
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

	// «Принять круг» (7.16) и оживление из архива (2.21–2.26, 2.28, 2.34).
	// Файл и пароль живут только на этом экране: уйти — значит выбрать файл
	// заново (2.24).
	const GB = 1024 * 1024 * 1024;
	// У сервера на своём адресе origin — пустая строка: не проверять как истину.
	const origin = $page.url.searchParams.get('origin') ?? '';

	type Step = 'choose' | 'password' | 'review' | 'upload' | 'done';
	let step = $state<Step>('choose');
	let serverName = $state('');
	let host = $state(displayHost(origin));
	let freeBytes = $state<number | undefined>();
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

	const need = $derived(archive?.bytes ?? 0);
	const circleName = $derived(String(archive?.manifest.circle_name ?? ''));
	const invitable = $derived(Math.max(0, (archive?.emails.length ?? 1) - 1));
	const srv = $derived(serverName ? `«${serverName}»` : 'этого сервера');

	onMount(async () => {
		const sessions = await loadSessions();
		const s = sessions.find((x) => x.origin === origin);
		serverName = s?.name ?? '';
		try {
			freeBytes = (await fetchReviveRoom(origin, 0)).room_bytes;
		} catch {
			/* без «свободно» */
		}
	});

	async function pick(files: File[]) {
		const f = files[0];
		if (!f) return;
		error = '';
		file = f;
		try {
			opened = await openFile(f);
			if (opened.encrypted) {
				step = 'password';
			} else {
				await load(opened.src);
			}
		} catch (err) {
			error = 'Это не файл архива круга Wynd.';
			console.warn('revive: open', err);
		}
	}

	async function openEncrypted() {
		if (!opened || !password.trim()) return;
		busy = true;
		passwordWrong = false;
		busyNote = 'Открываем файл…';
		try {
			await load(await unlock(opened, password.trim()));
		} catch (err) {
			if (err instanceof WrongPasswordError) passwordWrong = true;
			else error = 'Файл не открылся: он повреждён или это не архив круга Wynd.';
		} finally {
			busy = false;
			busyNote = '';
		}
	}

	async function load(src: Parameters<typeof loadArchive>[0]) {
		archive = await loadArchive(src);
		room = await fetchReviveRoom(origin, archive.bytes);
		step = 'review';
	}

	async function askPlace() {
		if (!archive) return;
		busy = true;
		error = '';
		try {
			await requestRevivePlace(origin, Math.ceil(archive.bytes / GB) * GB, circleName);
			room = await fetchReviveRoom(origin, archive.bytes);
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
		} catch (err) {
			error = authErrorHint(err);
			step = 'review';
		}
	}

	function day(iso: string | undefined): string {
		return iso ? formatSessionDay(iso) : '';
	}
</script>

<FormLayout
	shell
	app
	title={step === 'done' ? circleName : step === 'choose' ? 'Принять круг' : 'Оживить круг'}
	onback={() => goUp('/settings/servers')}
>
	<FilePicker bind:this={picker} accept=".wynd" onfiles={(files) => void pick(files)} />
	{#if step === 'choose'}
		<!-- 7.16: две дороги — перенос по ссылке (план 53) и оживление из файла. -->
		<ServerRow
			name={serverName || host}
			subtitle={freeBytes !== undefined ? `${host} · свободно ${formatBytes(freeBytes)}` : host}
			card
			top={10}
		/>
		<SettingsRow
			class="mt-14"
			title="Перенести с другого сервера"
			subtitle="по ссылке переноса — сервер заберёт круг сам"
			divided="top"
			onclick={() => goto(`/settings/servers/accept/move?origin=${encodeURIComponent(origin)}`)}
		/>
		<SettingsRow
			title="Оживить из архива"
			subtitle="из файла круга на этом устройстве"
			divided="both"
			onclick={() => picker?.open()}
		/>
		<Hint>
			Ссылку и файл делает владелец круга в его настройках: ссылку — «Перенести на другой сервер»,
			файл — «Архив и очистка» → «Полный архив круга». Круг приедет со своим журналом, людьми и
			названием — здесь вы станете его владельцем.
		</Hint>
	{:else if step === 'password' && file}
		<!-- 2.21 / 2.25: файл расшифровывается на этом устройстве. -->
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
			<Hint class="ink">Пароль не подошёл. Проверьте раскладку и цифры.</Hint>
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
		<Hint class="mt-14 ctr-text">Без пароля файл не открыть — его не знает ни сервер, ни Wynd.</Hint>
	{:else if step === 'review' && archive && room}
		<!-- 2.22 и, когда круг не помещается, 2.23 / 2.24 / 2.26. -->
		<Label>Сервер</Label>
		<ServerRow
			name={serverName || host}
			subtitle={room.fits
				? `${host} · свободно ${formatBytes(room.room_bytes)}`
				: `${host} · для круга есть ${formatBytes(room.room_bytes)}`}
			card
			top={4}
		/>
		<Hint>
			Круг будет жить здесь. Сервер хранит данные незашифрованными. Выбирайте тот, которому
			доверяете.
		</Hint>
		<Label class="mt-16">Архив</Label>
		<CircleRow
			initial={circleInitial(circleName)}
			name={circleName}
			preview={`${plural(archive.posts, WORD.post)} · ${plural(archive.members, WORD.member)} · ${formatBytes(archive.bytes)}`}
			time=""
		/>
		<Hint>
			Архив собран {formatAdminDayYear(archive.manifest.snapshot_at)} на сервере {displayHost(
				archive.manifest.server
			)}.{#if archive.lastEntry}{' '}Последняя запись — {formatEntryDate(archive.lastEntry)}.{/if}
		</Hint>
		{#if room.fits}
			{#if invitable > 0}
			<Label class="mt-16">Участники</Label>
			<SettingsRow
				title="Разослать приглашения"
				subtitle={`письмо на почту каждого из ${plural(invitable, WORD.memberGenitive)}`}
				divided="both"
			>
				{#snippet control()}
					<Switch bind:checked={sendInvites} label="Разослать приглашения" />
				{/snippet}
			</SettingsRow>
			<Hint>
				Почты в архиве — на день, когда его собрали. Кто войдёт сюда той же почтой, попадёт в своё
				имя; остальных позовёте сами.
			</Hint>
			{/if}
			<Button class="mt-14" variant="colored" onclick={() => void revive()}>
				Оживить круг здесь
			</Button>
			<Hint class="mt-8 ctr-text">Файл загружается частями и переживает обрыв связи.</Hint>
		{:else if room.request?.status === 'pending'}
			<StatusCard
				title="Место попросили"
				text={`${day(room.request.created_at)} вы попросили у администратора ${srv} ${formatBytes(room.request.requested_bytes)} для этого круга. Когда место дадут, здесь появится кнопка «Оживить круг здесь».`}
			/>
			<Hint class="mt-14">
				Можно уйти и вернуться позже: просьба останется, файл нужно будет выбрать заново.
			</Hint>
		{:else if room.request?.status === 'rejected'}
			<StatusCard
				title="В месте отказали"
				text={`${day(room.request.resolved_at ?? room.request.created_at)} администратор ${srv} отказал в ${formatBytes(room.request.requested_bytes)} для этого круга. Можно попросить меньше — кругу нужно ${formatBytes(need)} — или выбрать другой сервер.`}
			/>
			<Hint class="mt-14 ctr-text">
				<TextButton disabled={busy} onclick={() => void askPlace()}>Попросить ещё раз</TextButton>
			</Hint>
		{:else}
			<StatusCard
				title="Круг сюда не помещается"
				text={`Ему нужно ${formatBytes(need)}, а ${srv} для круга есть ${formatBytes(room.room_bytes)}. Выберите другой сервер или попросите место у администратора этого.`}
			/>
			<Button class="mt-14" variant="colored" loading={busy} onclick={() => void askPlace()}>
				Попросить место
			</Button>
		{/if}
	{:else if step === 'upload'}
		<Label>Загружаем круг</Label>
		<Meter value={total ? sent / total : 0} max={1} />
		<Hint class="mt-8">
			{formatBytes(sent)} из {formatBytes(total)}. Не закрывайте страницу, пока круг загружается.
		</Hint>
	{:else if step === 'done' && result && archive}
		<!-- 2.28 / 2.34. -->
		<StatusCard
			title="Круг на месте"
			text={result.verified
				? `${plural(result.posts, WORD.post)}, ${plural(result.names, WORD.name)}, ${formatBytes(result.bytes)} — сверено с файлом архива, ничего не потерялось.`
				: `${plural(result.posts, WORD.post)}, ${plural(result.names, WORD.name)}, ${formatBytes(result.bytes)} — с файлом архива сошлось не всё.`}
		/>
		{#if result.invited > 0}
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
				onclick={() => goto(`/circles/${result?.circle_id}/settings/members`)}
			/>
		{/if}
		<Hint>Кто войдёт на этот сервер почтой из архива, попадёт в своё имя и свои записи.</Hint>
		<Button class="mt-14" variant="colored" onclick={() => goto(`/circles/${result?.circle_id}`)}>
			Открыть круг
		</Button>
	{/if}
	{#if error}
		<Hint class="mt-12">{error}</Hint>
	{/if}
</FormLayout>
