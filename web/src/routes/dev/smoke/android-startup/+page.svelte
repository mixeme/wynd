<!-- План 50: локальный старт и стенд одного утверждённого исходника.
     Основа: действующий Desktop App.svelte, ПК1/ПК3/ПК4 на 54ecac07.
     Плашка ответа — ServerRow, план 56. b8014819 без плашки отклонён владельцем. -->
<script lang="ts">
	import { onMount } from 'svelte';
	import { startupAvailable, startupRequest } from '$lib/android/startup';
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
	import ServerRow from '$ui/data/ServerRow.svelte';
	import FilePicker from '$ui/forms/FilePicker.svelte';
	import QrScanner from '$ui/forms/QrScanner.svelte';
	import { decodeQrFromFile } from '$lib/auth/links';

	// Демонстрационные сцены допустимы только на Vite-стенде, никогда в APK.
	const native = startupAvailable() || !import.meta.env.DEV;
	const requested = native
		? 'start'
		: (new URLSearchParams(typeof location === 'undefined' ? '' : location.search).get('scene') ??
			'start');
	// Вариант ПК3 для приёмки доступен только на dev-стенде.
	const pc3CardPreview = import.meta.env.DEV && requested === 'pc3-card';
	const offlineScene =
		pc3CardPreview || requested === 'offline' || requested === 'offline-checking';
	let scanning = $state(false);
	let address = $state('');
	let photoError = $state('');
	let serverState = $state<'empty' | 'checking' | 'ready' | 'failed'>('empty');
	let photoPicker = $state<FilePicker>();
	let notice = $state('');
	let unavailable = $state(false);
	let knownName = $state('Сервер');
	let checkGeneration = 0;
	$effect(() => {
		address =
			requested === 'long'
				? 'family-photos-and-stories.example.org:8443'
				: offlineScene || ['checking', 'ready', 'error'].includes(requested)
					? 'home.example.org'
					: '';
		serverState =
			requested === 'checking' || requested === 'offline-checking'
				? 'checking'
				: requested === 'ready' || requested === 'long'
					? 'ready'
					: requested === 'error' || offlineScene
						? 'failed'
						: 'empty';
		photoError = '';
		unavailable = offlineScene;
		scanning = requested === 'scan';
	});
	const checking = $derived(serverState === 'checking');
	const serverName = $derived(
		native
			? knownName
			: unavailable
				? 'Дом Ани'
				: serverState === 'ready'
					? requested === 'long'
						? 'Семейный сервер на даче'
						: 'Дом Ани'
					: 'Сервер'
	);
	const serverHost = $derived.by(() => {
		const value = address.trim();
		try {
			return new URL(value.includes('://') ? value : `https://${value}`).host;
		} catch {
			return '';
		}
	});
	function resetCheck() {
		const pending = serverState !== 'empty';
		++checkGeneration;
		serverState = 'empty';
		knownName = 'Сервер';
		photoError = '';
		if (native && pending) void startupRequest('cancel').catch(() => {});
	}
	async function submit() {
		if (!address.trim() || checking) return;
		if (native) {
			const open = serverState === 'ready';
			const generation = ++checkGeneration;
			serverState = 'checking';
			try {
				const result = await startupRequest(open ? 'open' : 'check', address);
				if (generation !== checkGeneration) return;
				if (!open && result.preview) {
					knownName = result.preview.name;
					serverState = 'ready';
				}
			} catch {
				if (generation === checkGeneration) serverState = 'failed';
			}
			return;
		}
		// На стенде проверки сети и выбора источника нет. Пример ошибки — только для рендера.
		serverState = 'failed';
	}
	async function retry() {
		if (checking) return;
		serverState = 'checking';
		if (!native) return;
		try {
			await startupRequest('retry');
		} catch {
			serverState = 'failed';
		}
	}
	onMount(() => {
		if (!native) return;
		void startupRequest('state')
			.then((state) => {
				if (state.offline && state.selected) {
					unavailable = true;
					address = state.selected.origin;
					knownName = state.selected.name;
					serverState = 'failed';
				}
			})
			.catch(() => {
				/* При ошибке моста остаётся стартовая форма, выбор не меняется. */
			});
		const back = (event: Event) => {
			if (scanning) {
				scanning = false;
				event.preventDefault();
			}
		};
		window.addEventListener('wynd:android-back', back);
		return () => window.removeEventListener('wynd:android-back', back);
	});
	function readQr(value: string): boolean {
		address = value;
		scanning = false;
		resetCheck();
		return true;
	}
	async function readPhoto(file: File) {
		resetCheck();
		try {
			readQr(await decodeQrFromFile(file));
		} catch {
			photoError = 'Не удалось прочитать код — попробуйте другое фото';
		}
	}
</script>

{#snippet serverResponse()}
	<ServerRow
		name={serverName}
		subtitle={serverHost}
		detail="Сервер доступен"
		connection={checking ? 'checking' : serverState === 'failed' ? 'unavailable' : undefined}
		card
	/>
{/snippet}

{#snippet response()}
	<div class="server-slot" aria-live="polite" aria-busy={checking}>
		<!-- Существующая длинная плашка плана 56 задаёт запас по высоте в обеих ширинах.
		     Ни пустое состояние, ни ответ сервера не меняют размеры этого места. -->
		<div class="server-reserve" aria-hidden="true">
			<ServerRow
				name="Семейный сервер на даче"
				subtitle="family-photos-and-stories.example.org:8443"
				detail="Сервер доступен"
				card
			/>
		</div>
		<div class="server-state">
			{#if serverState !== 'empty'}
				{@render serverResponse()}
			{:else if photoError}<Hint>{photoError}</Hint>{/if}
		</div>
	</div>
{/snippet}

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
						{#if pc3CardPreview}
							{@render serverResponse()}
						{:else}
							<ScreenTitle centered style="font-size:22px;line-height:1.3;margin:0 16px 30px"
								>Сервер не отвечает</ScreenTitle
							>
							{@render response()}
						{/if}
						<Hint centered style="margin-top:14px"
							>Проверьте сеть. Если сеть есть — сервер выключен или переехал.</Hint
						>
						<Button style="margin-top:22px" stableHeight loading={checking} onclick={retry}
							>Повторить</Button
						>
						<Hint centered style="margin-top:26px"
							><TextButton
								onclick={() => {
									unavailable = false;
									address = '';
									resetCheck();
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
							oninput={resetCheck}
							onkeydown={(event: KeyboardEvent) => {
								if (event.key === 'Enter') submit();
							}}
						/>
						{@render response()}
						<Button
							style="margin-top:16px"
							stableHeight
							disabled={!address.trim()}
							loading={checking}
							onclick={submit}>Дальше</Button
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
	.server-slot {
		position: relative;
	}
	.server-reserve {
		visibility: hidden;
		overflow: hidden;
	}
	.server-state {
		position: absolute;
		inset: 0;
		overflow: auto;
	}
</style>
