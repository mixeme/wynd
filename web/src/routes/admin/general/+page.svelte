<script lang="ts">
	import { onMount } from 'svelte';
	import AdminSection from '$ui/admin/AdminSection.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Input from '$ui/forms/Input.svelte';
	import TextButton from '$ui/forms/TextButton.svelte';
	import SectionLabel from '$ui/data/SectionLabel.svelte';
	import AdminWideLayout from '$lib/layouts/AdminWideLayout.svelte';
	import { authErrorHint } from '$lib/auth/auth';
	import { displayHost } from '$lib/auth/origin';
	import {
		changeAdminPassword,
		fetchAccess,
		fetchSmtp,
		saveAccess,
		saveSmtp,
		sendSmtpTest,
		serverCaption
	} from '$lib/admin/admin';

	const label = 'margin:0;width:88px;flex:0 0 auto';
	const row = 'display:flex;align-items:center;gap:10px';

	let currentPassword = $state('');
	let newPassword = $state('');
	let name = $state('');
	let publicUrl = $state('');
	let host = $state('');
	let port = $state(587);
	let username = $state('');
	let smtpPassword = $state('');
	let from = $state('');
	let configured = $state(false);
	let testTo = $state('');
	let server = $state('');
	let error = $state('');
	let notice = $state('');
	let loading = $state(true);
	let savingPassword = $state(false);
	let sending = $state(false);

	async function persistName() {
		const trimmed = name.trim();
		if (!trimmed) return;
		error = '';
		try {
			await saveAccess({ name: trimmed });
			server = await serverCaption();
		} catch (err) {
			error = authErrorHint(err);
		}
	}

	async function persistUrl() {
		error = '';
		try {
			await saveAccess({ public_url: publicUrl.trim() });
			server = await serverCaption();
		} catch (err) {
			error = authErrorHint(err);
		}
	}

	async function persistSmtp(opts?: { quiet?: boolean }): Promise<boolean> {
		const trimmedHost = host.trim();
		const trimmedFrom = from.trim();
		if (!trimmedHost || !trimmedFrom) return false;
		error = '';
		if (!opts?.quiet) notice = '';
		try {
			await saveSmtp({
				host: trimmedHost,
				port: Number(port) || 587,
				username: username.trim(),
				password: smtpPassword,
				from: trimmedFrom
			});
			configured = true;
			smtpPassword = '';
			if (!opts?.quiet) notice = 'Сохранено';
			return true;
		} catch (err) {
			error = authErrorHint(err);
			return false;
		}
	}

	async function savePassword() {
		if (!currentPassword || !newPassword) {
			error = 'Введите текущий и новый пароль';
			notice = '';
			return;
		}
		savingPassword = true;
		error = '';
		notice = '';
		try {
			await changeAdminPassword(currentPassword, newPassword);
			currentPassword = '';
			newPassword = '';
			notice = 'Пароль сохранён';
		} catch (err) {
			error = authErrorHint(err);
		} finally {
			savingPassword = false;
		}
	}

	async function sendTest() {
		const to = testTo.trim();
		if (!to) {
			error = 'Укажите адрес, куда отправить письмо';
			notice = '';
			return;
		}
		error = '';
		notice = '';
		sending = true;
		try {
			const saved = await persistSmtp({ quiet: true });
			if (!saved) {
				if (!error) error = 'Сначала укажите хост и адрес отправителя';
				return;
			}
			await sendSmtpTest(to);
			notice = 'Письмо отправлено';
		} catch (err) {
			error = authErrorHint(err);
		} finally {
			sending = false;
		}
	}

	onMount(async () => {
		try {
			const [access, smtp, caption] = await Promise.all([
				fetchAccess(),
				fetchSmtp(),
				serverCaption()
			]);
			name = access.name;
			publicUrl = displayHost(access.public_url ?? '');
			host = smtp.host;
			port = smtp.port || 587;
			username = smtp.username;
			from = smtp.from;
			configured = smtp.configured;
			server = caption;
		} catch (err) {
			error = authErrorHint(err);
		} finally {
			loading = false;
		}
	});
</script>

<AdminWideLayout app active="Общие" {server}>
	<AdminSection title="Общие">
		{#if loading}
			<Hint>Загрузка…</Hint>
		{:else}
			<div style="display:flex;gap:44px">
				<div style="flex:1;min-width:0">
					<SectionLabel style="margin:0 0 10px">Пароль администратора</SectionLabel>
					<div style="font-size:12.5px;color:var(--muted);margin-bottom:12px;line-height:1.5">
						Отдельный пароль панели, не тот, которым входят в круги. Сбросить можно через консоль.
					</div>
					<div style="{row};margin-bottom:10px">
						<SectionLabel style={label}>Текущий</SectionLabel>
						<Input admin style="flex:1" type="password" bind:value={currentPassword} />
					</div>
					<div style="{row};margin-bottom:12px">
						<SectionLabel style={label}>Новый</SectionLabel>
						<Input admin style="flex:1" type="password" bind:value={newPassword} />
					</div>
					<TextButton
						variant="adminBox"
						style="font-weight:600"
						loading={savingPassword}
						onclick={() => void savePassword()}>Сохранить</TextButton
					>
					<SectionLabel style="margin:28px 0 10px">Имя и адрес</SectionLabel>
					<div style="{row};margin-bottom:10px">
						<SectionLabel style={label}>Имя</SectionLabel>
						<Input admin style="flex:1" bind:value={name} onchange={() => void persistName()} />
					</div>
					<div style={row}>
						<SectionLabel style={label}>Адрес</SectionLabel>
						<Input
							admin
							mono
							style="flex:1"
							bind:value={publicUrl}
							onchange={() => void persistUrl()}
						/>
					</div>
					<div style="font-size:11.5px;color:var(--faint);margin-top:10px;line-height:1.6">
						Так сервер назван в приложении. Адрес люди видят второй строкой и почти никогда не
						набирают.
					</div>
				</div>
				<div style="flex:1;min-width:0">
					<SectionLabel style="margin:0 0 10px">Почта</SectionLabel>
					<div style="font-size:12.5px;color:var(--muted);margin-bottom:12px;line-height:1.5">
						Люди входят по коду из письма. Без настройки SMTP письмо с кодом не отправится.
					</div>
					<div style="display:flex;flex-direction:column;gap:10px">
						<div style={row}>
							<SectionLabel style={label}>Хост</SectionLabel>
							<Input
								admin
								mono
								style="flex:1"
								bind:value={host}
								onchange={() => void persistSmtp()}
							/>
						</div>
						<div style={row}>
							<SectionLabel style={label}>Порт</SectionLabel>
							<Input
								admin
								style="width:72px"
								type="number"
								bind:value={port}
								onchange={() => void persistSmtp()}
							/>
							<Hint style="margin:0;white-space:nowrap">587 или 465</Hint>
						</div>
						<div style={row}>
							<SectionLabel style={label}>Логин</SectionLabel>
							<Input
								admin
								mono
								style="flex:1"
								bind:value={username}
								onchange={() => void persistSmtp()}
							/>
						</div>
						<div style={row}>
							<SectionLabel style={label}>Пароль</SectionLabel>
							<Input
								admin
								style="flex:1"
								type="password"
								placeholder={configured ? 'не менять' : ''}
								bind:value={smtpPassword}
								onchange={() => void persistSmtp()}
							/>
						</div>
						<div style={row}>
							<SectionLabel style={label}>От кого</SectionLabel>
							<Input
								admin
								mono
								style="flex:1"
								bind:value={from}
								onchange={() => void persistSmtp()}
							/>
						</div>
					</div>
					<SectionLabel style="margin:28px 0 8px">Проверочное письмо</SectionLabel>
					<div style="display:flex;align-items:center;gap:12px">
						<Input admin style="flex:1" placeholder="куда" bind:value={testTo} />
						<TextButton
							variant="adminBox"
							style="font-weight:600"
							loading={sending}
							onclick={() => void sendTest()}>Отправить</TextButton
						>
					</div>
				</div>
			</div>
			{#if sending}
				<Hint style="margin-top:12px">Отправляем…</Hint>
			{:else if notice}
				<Hint style="margin-top:12px">{notice}</Hint>
			{/if}
			{#if error}
				<Hint style="margin-top:12px">{error}</Hint>
			{/if}
		{/if}
	</AdminSection>
</AdminWideLayout>
