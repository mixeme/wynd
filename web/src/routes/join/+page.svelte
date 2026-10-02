<script lang="ts">
	import { goUp } from '$lib/navigation/up';
	import FilePicker from '$ui/forms/FilePicker.svelte';
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import Button from '$ui/forms/Button.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Input from '$ui/forms/Input.svelte';
	import Label from '$ui/forms/Label.svelte';
	import ServerRow from '$ui/data/ServerRow.svelte';
	import FormLayout from '$lib/layouts/FormLayout.svelte';
	import {
		authErrorHint,
		fetchInstance,
		flowForJoin,
		sendAuthCode,
		type InstanceInfo
	} from '$lib/auth/auth';
	import { decodeQrFromFile, foreignWyndLinkOrigin, parseWyndLink } from '$lib/auth/links';
	import { displayHost, resolveServerOrigin } from '$lib/auth/origin';
	import { INVALID_EMAIL_HINT, isValidParticipantEmail } from '$lib/auth/email';
	import { loadPendingAuth, savePendingAuth } from '$lib/auth/pending';
	import { appVersion } from '$lib/appinfo';
	import { sourceUrl } from '$lib/instance/source.svelte';

	let address = $state('');
	let email = $state('');
	let origin = $state('');
	let instance = $state<InstanceInfo | undefined>();
	let loading = $state(false);
	let checking = $state(false);
	let error = $state('');
	let linkPicker: FilePicker | undefined = $state();
	let linkText = $state('');

	// Из «Серверов» («Добавить сервер») назад — туда же: «/» у вошедшего
	// перекидывает в список кругов, и путь обратно терялся.
	const backHref = $derived(
		page.url.searchParams.get('from') === 'servers' ? '/settings/servers' : '/'
	);

	const blocked = $derived(instance ? instance.registration_mode !== 'open' : false);

	async function checkServer() {
		error = '';
		instance = undefined;
		const resolved = resolveServerOrigin(address);
		if (!resolved && !address.trim()) {
			origin = '';
			return;
		}
		checking = true;
		try {
			const info = await fetchInstance(resolved);
			origin = resolved;
			instance = info;
		} catch {
			error = 'Сервер не отвечает — проверьте адрес';
			origin = '';
		} finally {
			checking = false;
		}
	}

	onMount(() => {
		const pending = loadPendingAuth();
		if (pending?.flow === 'register') {
			email = pending.email;
			if (pending.origin) {
				address = displayHost(pending.origin);
				void checkServer();
				return;
			}
		}
		if (typeof window !== 'undefined') {
			address = window.location.host;
			void checkServer();
		}
	});

	const foreignLinkError = (origin: string) =>
		`Ссылка ведёт на другой сервер (${origin}) — откройте её там`;

	function onAddressPaste(event: ClipboardEvent) {
		const text = event.clipboardData?.getData('text') ?? '';
		const foreign = foreignWyndLinkOrigin(text);
		if (foreign) {
			event.preventDefault();
			error = foreignLinkError(foreign);
			return;
		}
		const path = parseWyndLink(text);
		if (!path) return;
		event.preventDefault();
		goto(path);
	}

	// Своё поле для ссылки (план 46, C5): в PWA ссылку из мессенджера
	// приложение не перехватывает — её копируют и вставляют сюда.
	function openLinkText(text: string) {
		const trimmed = text.trim();
		if (!trimmed) return;
		error = '';
		const foreign = foreignWyndLinkOrigin(trimmed);
		if (foreign) {
			error = foreignLinkError(foreign);
			return;
		}
		const path = parseWyndLink(trimmed);
		if (path) goto(path);
		else error = 'Это не ссылка-приглашение Wynd';
	}

	function onLinkPaste(event: ClipboardEvent) {
		const text = event.clipboardData?.getData('text') ?? '';
		if (!text.trim()) return;
		event.preventDefault();
		linkText = text.trim();
		openLinkText(linkText);
	}


	async function onLinkImageSelected(file: File) {
		error = '';
		try {
			const text = await decodeQrFromFile(file);
			const path = parseWyndLink(text);
			const foreign = foreignWyndLinkOrigin(text);
			if (path) goto(path);
			else if (foreign) error = foreignLinkError(foreign);
			else error = 'В коде нет ссылки Wynd';
		} catch {
			error = 'Не удалось прочитать код — попробуйте другое фото';
		}
	}

	async function onSubmit() {
		error = '';
		if (!instance) return;
		const trimmed = email.trim();
		if (!trimmed) {
			error = 'Введите почту';
			return;
		}
		if (!isValidParticipantEmail(trimmed)) {
			error = INVALID_EMAIL_HINT;
			return;
		}
		// Сервер новых не принимает — но своя учётка на нём входит почтой
		// (план 46, C4): закрыт приём новых, а не вход своих.
		const joinFlow = flowForJoin(instance.registration_mode, false);
		const flow = joinFlow === 'closed' ? 'login' : joinFlow;
		loading = true;
		try {
			const pending = {
				origin,
				email: trimmed,
				flow,
				instanceName: instance.name,
				codeDelivery: instance.code_delivery ?? 'mail'
			};
			await sendAuthCode(pending);
			savePendingAuth({ ...pending, codeSentAt: Date.now() });
			goto('/auth/code');
		} catch (err) {
			error = authErrorHint(err);
		} finally {
			loading = false;
		}
	}

	const openSubtitle = $derived(
		instance
			? `${displayHost(origin)} · открыт для новых · Wynd ${instance.version || appVersion}`
			: ''
	);
	const blockedSubtitle = $derived('только по приглашению');
</script>

<FormLayout shell app title="Без приглашения" onback={() => goUp(backHref)}>
	{#if !blocked}
		<Hint>
			Почта живёт на одном сервере. Общей на весь Wynd не бывает: серверы друг о друге не
			знают.
		</Hint>
	{/if}
	<Label>Адрес сервера</Label>
	<Input
		active
		mono
		small
		type="text"
		spellcheck="false"
		bind:value={address}
		onchange={checkServer}
		onblur={checkServer}
		onpaste={onAddressPaste}
	/>
	{#if checking}
		<Hint class="mt-8">Проверяем сервер…</Hint>
	{:else if instance}
		<ServerRow
			name={instance.name}
			subtitle={blocked ? blockedSubtitle : openSubtitle}
			variant={blocked ? 'warn' : 'ok'}
			card
		/>
	{/if}
	{#if blocked}
		<Hint class="mt-14">
			Сервер новых не принимает. Если у вас здесь уже есть учётка — войдите своей почтой.
		</Hint>
		<Label class="mt-16">Почта</Label>
		<Input active type="email" autocomplete="email" bind:value={email} />
		<Button {loading} onclick={onSubmit}>Войти</Button>
	{:else}
		<Label class="mt-16">Почта</Label>
		<Input active type="email" autocomplete="email" bind:value={email} />
		<Hint>
			Пришлём код. Пароля нет: почта понадобится, только чтобы вернуться на другом
			устройстве.
		</Hint>
		<Button {loading} disabled={!instance} onclick={onSubmit}>Получить код</Button>
		<Hint>
			Сервер хранит данные незашифрованными. Выбирайте сервер, которому доверяете, или
			<a class="under" href={sourceUrl(origin)}>поднимите свой</a>.
		</Hint>
	{/if}
	<Label class="mt-22">Есть приглашение</Label>
	<Input
		active
		mono
		small
		type="text"
		spellcheck="false"
		placeholder="вставьте ссылку"
		bind:value={linkText}
		onpaste={onLinkPaste}
		onchange={() => openLinkText(linkText)}
	/>
	{#if blocked}
		<Hint>
			Ссылку в круг даёт любой его участник, на сервер — тот, кто его держит.
		</Hint>
	{/if}
	<!-- Камерой — как на «Приглашении» (2.17); фото — если код прислали картинкой. -->
	<Button class="mt-14" variant="ghost" onclick={() => goto('/invite/scan?from=join')}
		>Сканировать QR-код</Button
	>
	<Button variant="ghost" onclick={() => linkPicker?.open()}>Фото с QR-кодом</Button>
	{#if error}
		<Hint class="mt-12">{error}</Hint>
	{/if}
</FormLayout>

<FilePicker bind:this={linkPicker} accept="image/*" onfiles={([file]) => void onLinkImageSelected(file)} />
