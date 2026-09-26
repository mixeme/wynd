<script lang="ts">
	import { onMount, tick } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import Hint from '$ui/forms/Hint.svelte';
	import Input from '$ui/forms/Input.svelte';
	import SectionLabel from '$ui/data/SectionLabel.svelte';
	import Button from '$ui/forms/Button.svelte';
	import TextButton from '$ui/forms/TextButton.svelte';
	import AdminSection from '$ui/admin/AdminSection.svelte';
	import StatusIcon from '$ui/admin/StatusIcon.svelte';
	import AdminWideLayout from '$lib/layouts/AdminWideLayout.svelte';
	import {
		authErrorHint,
		completeBootstrap,
		fetchInstance,
		probeBootstrapSmtp
	} from '$lib/auth/auth';
	import { isLoopbackPublicURL } from '$lib/auth/origin';

	const token = $derived(page.url.searchParams.get('token') ?? '');
	const cardStyle = 'panel pad-16';
	const descStyle = 'note mb-12 lh-15';
	const sideLabel = 'm-0 sz-12 nowrap';
	const sideLabelRight = 'm-0 ml-12 sz-12 nowrap';
	const fillInput = 'fill';

	let instanceName = $state('');
	let publicUrl = $state('');
	let smtpHost = $state('');
	let smtpPort = $state(587);
	let smtpUsername = $state('');
	let smtpPassword = $state('');
	let smtpFrom = $state('');
	let password = $state('');
	let isLoopback = $state(true);
	let loading = $state(false);
	let smtpTesting = $state(false);
	let smtpCheckedKey = $state('');
	let error = $state('');
	let ready = $state(false);
	let alreadyDone = $state(false);
	let errorBox = $state<HTMLDivElement | undefined>();

	const smtpKey = $derived(
		`${smtpHost}\0${smtpPort}\0${smtpUsername}\0${smtpPassword}\0${smtpFrom}`
	);
	const smtpOk = $derived(smtpCheckedKey !== '' && smtpCheckedKey === smtpKey);
	const loopbackNow = $derived(publicUrl.trim() ? isLoopbackPublicURL(publicUrl) : isLoopback);

	// Ошибка стоит над кнопкой, но на телефоне кнопку и её окрестность
	// закрывают клавиатура и панель браузера: без прокрутки нажатие выглядит
	// как «ничего не произошло».
	$effect(() => {
		if (!error) return;
		void tick().then(() => errorBox?.scrollIntoView({ block: 'center', behavior: 'smooth' }));
	});

	onMount(async () => {
		try {
			const instance = await fetchInstance('');
			isLoopback = instance.loopback;
			if (instance.bootstrapped) {
				alreadyDone = true;
			}
		} catch {
			error = 'Сервер не отвечает';
		} finally {
			ready = true;
		}
	});

	async function testSmtp() {
		error = '';
		smtpCheckedKey = '';
		if (!token) {
			error = 'Нет токена в ссылке — откройте URL из лога установки';
			return;
		}
		const host = smtpHost.trim();
		if (!host) {
			error = 'Укажите хост почтового сервера';
			return;
		}
		smtpTesting = true;
		const checked = smtpKey;
		try {
			await probeBootstrapSmtp({
				token,
				host,
				port: Number(smtpPort) || 587,
				username: smtpUsername.trim(),
				smtp_password: smtpPassword,
				from: smtpFrom.trim()
			});
			smtpCheckedKey = checked;
		} catch (err) {
			error = authErrorHint(err);
		} finally {
			smtpTesting = false;
		}
	}

	async function onSubmit() {
		error = '';
		smtpCheckedKey = '';
		if (!token) {
			error = 'Нет токена в ссылке — откройте URL из лога установки';
			return;
		}
		if (!password) {
			error = 'Введите пароль администратора';
			return;
		}
		const trimmedHost = smtpHost.trim();
		const trimmedFrom = smtpFrom.trim();
		const smtpComplete = Boolean(trimmedHost && trimmedFrom && smtpPassword);
		if (!loopbackNow && !smtpComplete) {
			error = 'Укажите SMTP: хост, адрес отправителя и пароль';
			return;
		}
		loading = true;
		try {
			await completeBootstrap({
				token,
				instance_name: instanceName.trim(),
				password,
				public_url: publicUrl.trim(),
				host: smtpComplete ? trimmedHost : '',
				port: Number(smtpPort) || 587,
				username: smtpComplete ? smtpUsername.trim() : '',
				smtp_password: smtpComplete ? smtpPassword : '',
				from: smtpComplete ? trimmedFrom : ''
			});
			goto('/admin/check');
		} catch (err) {
			error = authErrorHint(err);
		} finally {
			loading = false;
		}
	}
</script>

<AdminWideLayout app height="auto" nav={false}>
	<AdminSection title="Сервер поднят">
		{#if !ready}
			<Hint>Проверяем сервер…</Hint>
		{:else if alreadyDone}
			<Hint>Администратор уже создан.</Hint>
			<Button onclick={() => goto('/admin')}>Открыть панель</Button>
		{:else}
			<div class="col gap-16">
				<div class="flex gap-16 stretch col-narrow">
					<div class="grow min0 {cardStyle}">
						<div class="bold mb-6">1 · Пароль администратора</div>
						<div class={descStyle}>
							Пароль для доступа в панель администратора. Можно сбросить через консоль.
						</div>
						<Input
							admin
							type="password"
							autocomplete="new-password"
							bind:value={password}
							class="fill"
						/>
					</div>
					<div class="grow min0 {cardStyle}">
						<div class="bold mb-12">2 · Имя и адрес сервера</div>
						<div class="form-grid">
							<SectionLabel raw class={sideLabel}>Имя</SectionLabel>
							<Input
								admin
								type="text"
								autocomplete="organization"
								bind:value={instanceName}
								placeholder="Дом Ани"
								class={fillInput}
							/>
							<SectionLabel raw class={sideLabel}>Адрес</SectionLabel>
							<Input
								admin
								mono
								type="text"
								autocomplete="url"
								bind:value={publicUrl}
								placeholder="home.example.org"
								class={fillInput}
							/>
						</div>
					</div>
				</div>
				<div class={cardStyle}>
					<div class="bold mb-6">3 · Почта</div>
					<div class={descStyle}>
						{#if loopbackNow}
							На этом компьютере код входа пишется в окно сервера. Почту можно не указывать и настроить позже.
						{:else}
							Люди входят по коду из письма. Без настройки SMTP письмо с кодом не отправится.
						{/if}
					</div>
					<div class="form-grid-2">
						<SectionLabel raw class={sideLabel}>Хост</SectionLabel>
						<Input admin mono class={fillInput} bind:value={smtpHost} />
						<SectionLabel raw class={sideLabelRight}>Порт</SectionLabel>
						<div class="flex-mid gap-8 min0">
							<Input admin class="w72 flex-none" type="number" bind:value={smtpPort} />
							<Hint class="m-0 nowrap">587 или 465</Hint>
						</div>
						<SectionLabel raw class={sideLabel}>Логин</SectionLabel>
						<Input admin mono class={fillInput} bind:value={smtpUsername} />
						<SectionLabel raw class={sideLabelRight}>Пароль</SectionLabel>
						<Input admin class={fillInput} type="password" bind:value={smtpPassword} />
						<SectionLabel raw class={sideLabel}>От кого</SectionLabel>
						<Input admin mono class={fillInput} bind:value={smtpFrom} />
						<div class="grid-end flex-mid gap-8">
							{#if smtpTesting}
								<span role="status" class="note nowrap">
									Проверяем
								</span>
							{:else if smtpOk}
								<span role="status" aria-label="Вход принят">
									<StatusIcon status="ok" class="mt-0" />
								</span>
							{/if}
							<TextButton
								variant="adminBox"
								class="bold nowrap"
								disabled={smtpTesting || loading}
								onclick={() => void testSmtp()}
							>
								Проверить подключение
							</TextButton>
						</div>
					</div>
				</div>
			</div>
			{#if error}
				<div bind:this={errorBox} role="alert">
					<Hint class="mt-12">{error}</Hint>
				</div>
			{/if}
			<div class="mt-20">
				{#if loading}
					<span role="status" class="block note mb-8">
						Сохраняем
					</span>
				{/if}
				<Button class="fill m-0" disabled={loading} onclick={loading ? () => {} : onSubmit}>
					Сохранить и открыть панель
				</Button>
			</div>
		{/if}
	</AdminSection>
</AdminWideLayout>
