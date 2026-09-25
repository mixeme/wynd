<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
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

	let address = $state('');
	let email = $state('');
	let origin = $state('');
	let instance = $state<InstanceInfo | undefined>();
	let loading = $state(false);
	let checking = $state(false);
	let error = $state('');
	let linkInput: HTMLInputElement | undefined;

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

	async function openLinkPicker() {
		error = '';
		try {
			const text = await navigator.clipboard.readText();
			const path = parseWyndLink(text);
			if (path) {
				goto(path);
				return;
			}
		} catch {
			/* clipboard denied or empty */
		}
		linkInput?.click();
	}

	async function onLinkImageSelected() {
		const file = linkInput?.files?.[0];
		if (!file) return;
		error = '';
		try {
			const text = await decodeQrFromFile(file);
			const path = parseWyndLink(text);
			const foreign = foreignWyndLinkOrigin(text);
			if (path) goto(path);
			else if (foreign) error = foreignLinkError(foreign);
			else error = 'В коде нет ссылки Wynd';
		} catch (err) {
			error =
				err instanceof Error && err.message === 'no_detector'
					? 'Сканер QR недоступен в этом браузере — вставьте ссылку в поле'
					: 'Не удалось прочитать код — попробуйте другое фото';
		} finally {
			if (linkInput) linkInput.value = '';
		}
	}

	async function onSubmit() {
		error = '';
		if (!instance || blocked) return;
		const trimmed = email.trim();
		if (!trimmed) {
			error = 'Введите почту';
			return;
		}
		if (!isValidParticipantEmail(trimmed)) {
			error = INVALID_EMAIL_HINT;
			return;
		}
		const flow = flowForJoin(instance.registration_mode, false);
		if (flow === 'closed') return;
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

<FormLayout shell app title="Без приглашения" onback={() => goto('/')}>
	{#if !blocked}
		<Hint>
			Почта живёт на одном сервере. Общей на весь Wynd не бывает: серверы друг о друге не
			знают.
		</Hint>
	{/if}
	<Label style="margin-top:18px">Адрес сервера</Label>
	<Input
		active
		mono
		style="font-size:12.5px"
		type="text"
		spellcheck="false"
		bind:value={address}
		onchange={checkServer}
		onblur={checkServer}
		onpaste={onAddressPaste}
	/>
	{#if checking}
		<Hint style="margin-top:8px">Проверяем сервер…</Hint>
	{:else if instance}
		<ServerRow
			name={instance.name}
			subtitle={blocked ? blockedSubtitle : openSubtitle}
			variant={blocked ? 'warn' : 'ok'}
			card
		/>
	{/if}
	{#if blocked}
		<Hint style="margin-top:14px">
			Сервер жив и отвечает, но сам новых не принимает. Нужна ссылка — в круг или на сервер:
			первую даёт любой участник круга, вторую — тот, кто держит сервер.
		</Hint>
		<Button disabled onclick={() => {}}>Получить код</Button>
		<Hint centered style="margin-top:26px">
			Закрытый сервер выглядит так же,<br />но у него ссылок не выдают вовсе.
		</Hint>
	{:else}
		<Label style="margin-top:16px">Почта</Label>
		<Input active type="email" autocomplete="email" bind:value={email} />
		<Hint>
			Пришлём код. Пароля нет: почта понадобится, только чтобы вернуться на другом
			устройстве.
		</Hint>
		<Button {loading} disabled={!instance} onclick={onSubmit}>Получить код</Button>
		<Hint>
			Сервер хранит данные незашифрованными. Выбирайте сервер, которому доверяете, или
			<a class="under" href="https://github.com/mixeme/wynd">поднимите свой</a>.
		</Hint>
		<Button variant="ghost" style="margin-top:18px" onclick={openLinkPicker}>
			Открыть ссылку или QR
		</Button>
		<Hint centered style="margin-top:8px">Ссылку можно вставить в поле адреса выше</Hint>
	{/if}
	{#if error}
		<Hint style="margin-top:12px">{error}</Hint>
	{/if}
</FormLayout>

<input
	bind:this={linkInput}
	type="file"
	accept="image/*"
	hidden
	onchange={onLinkImageSelected}
/>
