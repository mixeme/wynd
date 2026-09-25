<script lang="ts">
	import { onMount } from 'svelte';
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
	const cardStyle =
		'border:1px solid var(--line);background:var(--card);border-radius:12px;padding:16px';
	const descStyle = 'font-size:12.5px;color:var(--muted);margin-bottom:12px;line-height:1.5';
	const sideLabel = 'margin:0;font-size:12.5px;white-space:nowrap';
	const sideLabelRight = 'margin:0 0 0 12px;font-size:12.5px;white-space:nowrap';
	const fillInput = 'width:100%;min-width:0;box-sizing:border-box';

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

	const smtpKey = $derived(
		`${smtpHost}\0${smtpPort}\0${smtpUsername}\0${smtpPassword}\0${smtpFrom}`
	);
	const smtpOk = $derived(smtpCheckedKey !== '' && smtpCheckedKey === smtpKey);
	const loopbackNow = $derived(publicUrl.trim() ? isLoopbackPublicURL(publicUrl) : isLoopback);

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
			<div style="display:flex;flex-direction:column;gap:16px">
				<div style="display:flex;gap:16px;align-items:stretch">
					<div style="flex:1;min-width:0;{cardStyle}">
						<div style="font-weight:600;margin-bottom:6px">1 · Пароль администратора</div>
						<div style={descStyle}>
							Пароль для доступа в панель администратора. Можно сбросить через консоль.
						</div>
						<Input
							admin
							type="password"
							autocomplete="new-password"
							bind:value={password}
							style="width:100%;box-sizing:border-box"
						/>
					</div>
					<div style="flex:1;min-width:0;{cardStyle}">
						<div style="font-weight:600;margin-bottom:12px">2 · Имя и адрес сервера</div>
						<div style="display:grid;grid-template-columns:auto minmax(0,1fr);gap:8px;align-items:center">
							<SectionLabel raw style={sideLabel}>Имя</SectionLabel>
							<Input
								admin
								type="text"
								autocomplete="organization"
								bind:value={instanceName}
								placeholder="Дом Ани"
								style={fillInput}
							/>
							<SectionLabel raw style={sideLabel}>Адрес</SectionLabel>
							<Input
								admin
								mono
								type="text"
								autocomplete="url"
								bind:value={publicUrl}
								placeholder="home.example.org"
								style={fillInput}
							/>
						</div>
					</div>
				</div>
				<div style={cardStyle}>
					<div style="font-weight:600;margin-bottom:6px">3 · Почта</div>
					<div style={descStyle}>
						{#if loopbackNow}
							На этом компьютере код входа пишется в окно сервера. Почту можно не указывать и настроить позже.
						{:else}
							Люди входят по коду из письма. Без настройки SMTP письмо с кодом не отправится.
						{/if}
					</div>
					<div
						style="display:grid;grid-template-columns:auto minmax(0,1fr) auto minmax(0,1fr);gap:8px;align-items:center"
					>
						<SectionLabel raw style={sideLabel}>Хост</SectionLabel>
						<Input admin mono style={fillInput} bind:value={smtpHost} />
						<SectionLabel raw style={sideLabelRight}>Порт</SectionLabel>
						<div style="display:flex;align-items:center;gap:8px;min-width:0">
							<Input
								admin
								style="width:72px;flex:none;box-sizing:border-box"
								type="number"
								bind:value={smtpPort}
							/>
							<Hint style="margin:0;white-space:nowrap">587 или 465</Hint>
						</div>
						<SectionLabel raw style={sideLabel}>Логин</SectionLabel>
						<Input admin mono style={fillInput} bind:value={smtpUsername} />
						<SectionLabel raw style={sideLabelRight}>Пароль</SectionLabel>
						<Input admin style={fillInput} type="password" bind:value={smtpPassword} />
						<SectionLabel raw style={sideLabel}>От кого</SectionLabel>
						<Input admin mono style={fillInput} bind:value={smtpFrom} />
						<div
							style="grid-column:3/-1;justify-self:end;display:flex;align-items:center;gap:8px"
						>
							{#if smtpTesting}
								<span role="status" style="font-size:12.5px;color:var(--muted);white-space:nowrap">
									Проверяем
								</span>
							{:else if smtpOk}
								<span role="status" aria-label="Вход принят">
									<StatusIcon status="ok" style="margin-top:0" />
								</span>
							{/if}
							<TextButton
								variant="adminBox"
								style="font-weight:600;white-space:nowrap"
								disabled={smtpTesting || loading}
								onclick={() => void testSmtp()}
							>
								Проверить подключение
							</TextButton>
						</div>
					</div>
				</div>
			</div>
			<div style="margin:20px 0 0">
				{#if loading}
					<span
						role="status"
						style="display:block;font-size:12.5px;color:var(--muted);margin-bottom:8px"
					>
						Сохраняем
					</span>
				{/if}
				<Button
					style="width:100%;margin:0"
					disabled={loading}
					onclick={loading ? () => {} : onSubmit}
				>
					Сохранить и открыть панель
				</Button>
			</div>
		{/if}
		{#if error}
			<Hint style="margin-top:12px">{error}</Hint>
		{/if}
	</AdminSection>
</AdminWideLayout>
