<!-- План 50: предварительный рендер, не локальная стартовая часть APK.
     Основа: действующий Desktop App.svelte, ПК1/ПК3/ПК4 на 54ecac07.
     Отличие первого запуска — две существующие QR-кнопки под «Дальше». -->
<script lang="ts">
	import { page } from '$app/state';
	import PlainLayout from '$lib/layouts/PlainLayout.svelte';
	import FormLayout from '$lib/layouts/FormLayout.svelte';
	import Mark from '$ui/Mark.svelte';
	import Logo from '$ui/Logo.svelte';
	import ScreenTitle from '$ui/forms/ScreenTitle.svelte';
	import Label from '$ui/forms/Label.svelte';
	import Input from '$ui/forms/Input.svelte';
	import Button from '$ui/forms/Button.svelte';
	import TextButton from '$ui/forms/TextButton.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import FilePicker from '$ui/forms/FilePicker.svelte';
	import QrScanner from '$ui/forms/QrScanner.svelte';
	import { decodeQrFromFile } from '$lib/auth/links';

	const requested = $derived(page.url.searchParams.get('scene') ?? 'start');
	let scanning = $state(false);
	let address = $state('');
	let error = $state('');
	let photoPicker = $state<FilePicker>();
	let notice = $state('');
	let unavailable = $state(false);
	$effect(() => {
		address = requested === 'error' ? 'home.example.org' : '';
		error =
			requested === 'error'
				? 'Сервер Wynd по этому адресу не отвечает. Проверьте адрес и сеть.'
				: '';
		unavailable = requested === 'offline';
		scanning = requested === 'scan';
	});
	function submit() {
		if (!address.trim()) return;
		// На стенде проверки сети и выбора источника нет. Пример ошибки — только для рендера.
		error = 'Сервер Wynd по этому адресу не отвечает. Проверьте адрес и сеть.';
	}
	function readQr(value: string): boolean {
		address = value;
		scanning = false;
		error = '';
		return true;
	}
	async function readPhoto(file: File) {
		try {
			readQr(await decodeQrFromFile(file));
		} catch {
			error = 'Не удалось прочитать код — попробуйте другое фото';
		}
	}
</script>

<div class="preview">
	{#if scanning}
		<FormLayout shell app title="Сканер" onback={() => (scanning = false)}>
			<QrScanner onread={readQr} onerror={(message) => (notice = message)} />
			<Hint>{notice || 'Наведите камеру на QR-код приглашения — он откроется сам.'}</Hint>
		</FormLayout>
	{:else}
		<PlainLayout shell app>
			<div class="startup-content">
				<div class="welcome">
					{#if unavailable}
						<div class="welcome-logo"><Logo /></div>
						<ScreenTitle centered style="font-size:22px;line-height:1.3;margin:0 16px 30px"
							>Сервер не отвечает</ScreenTitle
						>
						<Hint centered>Дом Ани · home.example.org</Hint>
						<Hint centered style="margin-top:14px"
							>Проверьте сеть. Если сеть есть — сервер выключен или переехал.</Hint
						>
						<Button style="margin-top:22px" onclick={() => {}}>Повторить</Button>
						<Hint centered style="margin-top:26px"
							><TextButton
								onclick={() => {
									unavailable = false;
									address = '';
									error = '';
								}}>Другой сервер</TextButton
							></Hint
						>
					{:else}
						<div class="welcome-mark"><Mark /></div>
						<ScreenTitle centered style="font-size:22px;line-height:1.3;margin:0 16px 30px"
							>Войти в Wynd</ScreenTitle
						>
						<Label>Адрес сервера или приглашение</Label>
						<Input
							bind:value={address}
							placeholder="wynd.example.org"
							aria-label="Адрес сервера или приглашение"
							type="text"
							spellcheck={false}
							autocomplete="off"
							autocapitalize="off"
							oninput={() => (error = '')}
							onkeydown={(event: KeyboardEvent) => {
								if (event.key === 'Enter') submit();
							}}
						/>
						{#if error}<Hint>{error}</Hint>{/if}
						<Button style="margin-top:16px" disabled={!address.trim()} onclick={submit}
							>Дальше</Button
						>
						<Button
							style="margin-top:16px"
							variant="ghost"
							onclick={() => {
								notice = '';
								scanning = true;
							}}>Сканировать QR-код</Button
						>
						<Button style="margin-top:16px" variant="ghost" onclick={() => photoPicker?.open()}
							>Фото с QR-кодом</Button
						>
					{/if}
				</div>
			</div>
		</PlainLayout>
	{/if}
	<FilePicker bind:this={photoPicker} accept="image/*" onfiles={([file]) => void readPhoto(file)} />
</div>

<style>
	/* Только стенд: без внешних полей галереи /dev. */
	.preview {
		position: fixed;
		inset: 0;
	}
	/* Основа — Desktop app.css; поля компонентов 16 px, без внешних полей окна. */
	.startup-content {
		flex: 1;
		min-height: 0;
		display: grid;
		grid-template-columns: minmax(0, 1fr);
		place-items: center;
		padding: 0;
		overflow: auto;
	}
	.welcome {
		width: 390px;
		max-width: 100%;
		padding-bottom: 38px;
	}
	.welcome-mark {
		width: 46px;
		margin: 0 auto 25px;
		color: var(--terracotta);
	}
	.welcome-logo {
		width: 140px;
		margin: 0 auto 30px;
		color: var(--ink);
	}
</style>
