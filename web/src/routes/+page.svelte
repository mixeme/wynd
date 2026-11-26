<script lang="ts">
	import ScreenTitle from '$ui/forms/ScreenTitle.svelte';
	import NewsBanner from '$ui/data/NewsBanner.svelte';
	import { androidUpdate } from '$lib/android/update.svelte';
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { androidInterfacesAvailable, openStartupScreen } from '$lib/android/interfaces';
	import { desktopBridge } from '$lib/desktop/bridge';
	import { returnDesktopToAddress } from '$lib/desktop/servers';
	import { navigateBack } from '$lib/navigation/up';
	import BackBar from '$ui/chrome/BackBar.svelte';
	import Button from '$ui/forms/Button.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Loading from '$ui/Loading.svelte';
	import Input from '$ui/forms/Input.svelte';
	import Label from '$ui/forms/Label.svelte';
	import Logo from '$ui/Logo.svelte';
	import ServerRow from '$ui/data/ServerRow.svelte';
	import {
		loadServerCard,
		registrationCaption,
		type ServerConnection
	} from '$lib/instance/serverCard';
	import PlainLayout from '$lib/layouts/PlainLayout.svelte';
	import { addressErrorHint, sendAuthCode, type InstanceInfo } from '$lib/auth/auth';
	import { displayHost, resolveServerOrigin } from '$lib/auth/origin';
	import { INVALID_EMAIL_HINT, isValidParticipantEmail } from '$lib/auth/email';
	import { loadPendingAuth, savePendingAuth } from '$lib/auth/pending';
	import { initSession, loadSessions } from '$lib/session/session.svelte';

	let address = $state('');
	let email = $state('');
	let origin = $state('');
	let instance = $state<InstanceInfo | undefined>();
	let loading = $state(false);
	// Пока не ясно, есть ли вход: у вошедшего форма входа мелькала перед
	// переходом в круги.
	let booting = $state(true);
	let checking = $state(false);
	let serverConnection = $state<ServerConnection>();
	let serverError = $state('');
	let checkGeneration = 0;
	let error = $state('');

	async function checkServer() {
		const generation = ++checkGeneration;
		serverError = '';
		const resolved = resolveServerOrigin(address);
		if (resolved !== origin) instance = undefined;
		if (!resolved && !address.trim()) {
			origin = '';
			checking = false;
			serverConnection = undefined;
			return;
		}
		checking = true;
		serverConnection = 'checking';
		const known = instance;
		const card = await loadServerCard(resolved);
		if (generation !== checkGeneration) return;
		origin = resolved;
		instance = card.info ?? known;
		serverConnection = card.connection;
		if (card.connection === 'unavailable') {
			serverError = instance
				? 'Сервер не отвечает — попробуйте позже'
				: 'Сервер не отвечает — проверьте адрес';
		}
		checking = false;
	}

	onMount(() => {
		// Системная «Назад» в Android на экране входа ведёт туда же, куда
		// стрелка, — к вводу адреса (план 50). Оверлеи закрывает общий
		// обработчик раньше и помечает событие отменённым.
		if (!androidInterfacesAvailable()) return;
		const back = (event: Event) => {
			if (event.defaultPrevented) return;
			event.preventDefault();
			void leaveToAddress();
		};
		window.addEventListener('wynd:android-back', back);
		return () => window.removeEventListener('wynd:android-back', back);
	});

	onMount(async () => {
		// «/?loading» держит экран загрузки: на быстром сервере его иначе не
		// разглядеть. Ничего не грузит и не ломает — просто не идёт дальше.
		if (new URL(window.location.href).searchParams.has('loading')) return;
		await initSession();
		const sessions = await loadSessions();
		if (sessions.length) {
			goto('/circles');
			return;
		}
		booting = false;
		const pending = loadPendingAuth();
		if (pending?.flow === 'login') {
			email = pending.email;
			if (pending.origin) {
				address = displayHost(pending.origin);
				void checkServer();
				return;
			}
		}
		// Ссылка подключения устройства (план 72): почта уже подставлена —
		// остаётся «Получить код» и код из письма. Незаметна без параметра.
		const linked =
			typeof window !== 'undefined'
				? new URL(window.location.href).searchParams.get('email')
				: null;
		if (!pending && linked && isValidParticipantEmail(linked.trim())) {
			email = linked.trim();
		}
		if (typeof window !== 'undefined') {
			address = displayHost('');
			void checkServer();
		}
	});

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
		loading = true;
		try {
			const pending = {
				origin,
				email: trimmed,
				flow: 'login' as const,
				instanceName: instance.name,
				codeDelivery: instance.code_delivery ?? 'mail'
			};
			await sendAuthCode(pending);
			savePendingAuth({ ...pending, codeSentAt: Date.now() });
			goto('/auth/code');
		} catch (err) {
			error = addressErrorHint(err);
		} finally {
			loading = false;
		}
	}

	// Только адрес: галочка уже говорит, что сервер на связи. «принимает вход»
	// читалось непонятно.
	const serverSubtitle = $derived(instance ? address : '');

	// Назад с экрана входа — к вводу адреса сервера (план 50). В окне — экран
	// оболочки «Войти в Wynd» с прежним адресом, в Android — стартовый экран
	// оболочки (черновик адреса восстанавливает он сам). В браузере стрелки нет.
	async function leaveToAddress() {
		if (androidInterfacesAvailable()) {
			try {
				await openStartupScreen();
				return;
			} catch {
				/* Мост не ответил — экран остаётся как есть. */
			}
		}
		const bridge = desktopBridge();
		if (bridge) {
			returnDesktopToAddress(bridge.origin);
			return;
		}
		await navigateBack();
	}
</script>

{#if booting}
	<PlainLayout shell app>
		<Loading logo />
	</PlainLayout>
{:else}
	<PlainLayout shell app>
		{#if androidUpdate.available && !androidUpdate.hidden}
			<NewsBanner
				version={androidUpdate.nextVersion}
				title={`Вышла версия ${androidUpdate.nextVersion}`}
				text="Обновить приложение"
				onclick={() => goto('/settings')}
				ondismiss={() => (androidUpdate.hidden = true)}
			/>
		{/if}
		{#if desktopBridge() || androidInterfacesAvailable()}
			<!-- Первый запуск: адрес подтверждён, экран входа — назад к вводу
		     адреса (план 50, находка владельца 20.11). Окно возвращается на
		     свой экран «Войти в Wynd» с прежним адресом, Android — на
		     стартовый экран оболочки, черновик адреса он восстановит сам.
		     В браузере «Назад» есть у самого браузера. -->
			<BackBar onback={() => void leaveToAddress()} />
		{/if}
		<div class="logo-wrap">
			<Logo />
		</div>
		<ScreenTitle class="mt-16" centered>Войти</ScreenTitle>
		<!-- Адрес не вводится: приложение открыто с этого сервера, он и есть сервер
	     входа. Другой сервер в окне — «Назад» к «Серверам»; в веб-приложении второй
	     сервер пока не добавить (C69). -->
		{#if instance}
			<ServerRow
				name={instance.name}
				subtitle={serverSubtitle}
				detail={registrationCaption(instance.registration_mode)}
				connection={serverConnection}
				card
				gdpr={instance.allow_name_erasure}
			/>
		{:else if checking || serverError}
			<!-- Имя сервера приходит с ответом; адрес известен сразу. Три строки
			     и в проверке, и после неё: иначе строка «Почта» прыгает вниз,
			     когда появляется режим регистрации. -->
			<ServerRow
				name="Сервер"
				subtitle={address || displayHost(origin)}
				connection={checking ? 'checking' : 'unavailable'}
				card
			/>
		{/if}
		{#if serverError}<Hint class="mt-8">{serverError}</Hint>{/if}
		<Label class="mt-16">Почта</Label>
		<Input active type="email" autocomplete="email" bind:value={email} />
		<Hint>Пришлём код для входа. Пароля нет.</Hint>
		<Button {loading} disabled={!instance} onclick={onSubmit}>Получить код</Button>
		<!-- Без приглашения — только на открытом сервере: иначе дверь вела в тупик.
	     Каждая ссылка — своей строкой с промежутком: вплотную в них промахивались. -->
		{#if instance?.registration_mode === 'open'}
			<Hint class="mt-16" centered>
				<a class="under" href="/join">Регистрация без приглашения</a>
			</Hint>
		{/if}
		{#if instance && instance.registration_mode !== 'closed'}
			<Hint class={instance?.registration_mode === 'open' ? 'mt-16' : 'mt-16'} centered>
				<a class="under" href="/invite?from=login">У меня есть приглашение</a>
			</Hint>
		{/if}
		<!-- Быстрое подключение: QR со «Подключить устройство» другого устройства (план 72). -->
		<Hint class="mt-16" centered>
			<a class="under" href="/invite/scan?from=login">Войти по QR-коду</a>
		</Hint>
		{#if error}
			<Hint class="mt-8" centered>{error}</Hint>
		{/if}
	</PlainLayout>
{/if}

<style>
	.logo-wrap {
		margin: 38px 0 0;
		display: flex;
		justify-content: center;
		color: var(--ink);
	}
</style>
