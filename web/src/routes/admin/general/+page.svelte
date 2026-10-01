<script lang="ts">
	import AdminField from '$ui/admin/AdminField.svelte';
	import { onMount } from 'svelte';
	import AdminSection from '$ui/admin/AdminSection.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Loading from '$ui/Loading.svelte';
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
			<Loading compact />
		{:else}
			<div class="flex gap-44">
				<div class="grow min0">
					<SectionLabel class="mt-0 mx-0 mb-10">Пароль администратора</SectionLabel>
					<div class="note mb-12 lh-15">
						Отдельный пароль панели, не тот, которым входят в круги. Сбросить можно через консоль.
					</div>
					<AdminField label="Текущий" width={88} class="mb-10">
						<Input admin class="grow" type="password" bind:value={currentPassword} />
					</AdminField>
					<AdminField label="Новый" width={88} class="mb-12">
						<Input admin class="grow" type="password" bind:value={newPassword} />
					</AdminField>
					<TextButton
						variant="adminBox"
						class="bold"
						loading={savingPassword}
						onclick={() => void savePassword()}>Сохранить</TextButton
					>
					<SectionLabel class="mt-28 mx-0 mb-10">Имя и адрес</SectionLabel>
					<AdminField label="Имя" width={88} class="mb-10">
						<Input admin class="grow" bind:value={name} onchange={() => void persistName()} />
					</AdminField>
					<AdminField label="Адрес" width={88}>
						<Input
							admin
							mono
							class="grow"
							bind:value={publicUrl}
							onchange={() => void persistUrl()}
						/>
					</AdminField>
					<div class="fine mt-10 lh-16">
						Так сервер назван в приложении. Адрес люди видят второй строкой и почти никогда не
						набирают.
					</div>
				</div>
				<div class="grow min0">
					<SectionLabel class="mt-0 mx-0 mb-10">Почта</SectionLabel>
					<div class="note mb-12 lh-15">
						Люди входят по коду из письма. Без настройки SMTP письмо с кодом не отправится.
					</div>
					<div class="col gap-10">
						<AdminField label="Хост" width={88}>
							<Input
								admin
								mono
								class="grow"
								bind:value={host}
								onchange={() => void persistSmtp()}
							/>
						</AdminField>
						<AdminField label="Порт" width={88} unit="587 или 465">
							<Input
								admin
								class="w72"
								type="number"
								bind:value={port}
								onchange={() => void persistSmtp()}
							/>
						</AdminField>
						<AdminField label="Логин" width={88}>
							<Input
								admin
								mono
								class="grow"
								bind:value={username}
								onchange={() => void persistSmtp()}
							/>
						</AdminField>
						<AdminField label="Пароль" width={88}>
							<Input
								admin
								class="grow"
								type="password"
								placeholder={configured ? 'не менять' : ''}
								bind:value={smtpPassword}
								onchange={() => void persistSmtp()}
							/>
						</AdminField>
						<AdminField label="От кого" width={88}>
							<Input
								admin
								mono
								class="grow"
								bind:value={from}
								onchange={() => void persistSmtp()}
							/>
						</AdminField>
					</div>
					<SectionLabel class="mt-28 mx-0 mb-8">Проверочное письмо</SectionLabel>
					<div class="flex-mid gap-12">
						<Input admin class="grow" placeholder="куда" bind:value={testTo} />
						<TextButton
							variant="adminBox"
							class="bold"
							loading={sending}
							onclick={() => void sendTest()}>Отправить</TextButton
						>
					</div>
				</div>
			</div>
			{#if sending}
				<Hint class="mt-12">Отправляем…</Hint>
			{:else if notice}
				<Hint class="mt-12">{notice}</Hint>
			{/if}
			{#if error}
				<Hint class="mt-12">{error}</Hint>
			{/if}
		{/if}
	</AdminSection>
</AdminWideLayout>
