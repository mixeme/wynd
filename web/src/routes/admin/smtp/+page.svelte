<script lang="ts">
	import { goto } from '$app/navigation';
	import { onMount } from 'svelte';
	import AdminSection from '$ui/admin/AdminSection.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Icon from '$ui/Icon.svelte';
	import Input from '$ui/forms/Input.svelte';
	import TextButton from '$ui/forms/TextButton.svelte';
	import SectionLabel from '$ui/data/SectionLabel.svelte';
	import AdminWideLayout from '$lib/layouts/AdminWideLayout.svelte';
	import { authErrorHint } from '$lib/auth/auth';
	import { fetchSmtp, saveSmtp, sendSmtpTest, serverCaption } from '$lib/admin/admin';

	let host = $state('');
	let port = $state(587);
	let username = $state('');
	let password = $state('');
	let from = $state('');
	let configured = $state(false);
	let testTo = $state('');
	let server = $state('');
	let error = $state('');
	let notice = $state('');
	let loading = $state(true);
	let saving = $state(false);

	async function persist() {
		const trimmedHost = host.trim();
		const trimmedFrom = from.trim();
		if (!trimmedHost || !trimmedFrom) return;
		saving = true;
		error = '';
		notice = '';
		try {
			await saveSmtp({
				host: trimmedHost,
				port: Number(port) || 587,
				username: username.trim(),
				password,
				from: trimmedFrom
			});
			configured = true;
			password = '';
			notice = 'Сохранено';
		} catch (err) {
			error = authErrorHint(err);
		} finally {
			saving = false;
		}
	}

	async function sendTest() {
		const to = testTo.trim();
		if (!to) return;
		error = '';
		notice = '';
		try {
			await sendSmtpTest(to);
			notice = 'Письмо отправлено';
		} catch (err) {
			error = authErrorHint(err);
		}
	}

	onMount(async () => {
		try {
			const [smtp, caption] = await Promise.all([fetchSmtp(), serverCaption()]);
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

<AdminWideLayout app active="Проверка" {server}>
	<AdminSection>
		<TextButton
			variant="admin"
			style="font-size:11.5px;color:var(--faint);margin-bottom:8px;display:flex;align-items:center;gap:8px"
			onclick={() => goto('/admin/check')}
		>
			<Icon name="back" size="sm" />
			Проверка
		</TextButton>
		<h4 style="margin-bottom:8px">Почта</h4>
		{#if loading}
			<Hint>Загрузка…</Hint>
		{:else}
			<div style="font-size:12.5px;color:var(--muted);margin-bottom:16px;line-height:1.6;max-width:620px">
				Внешний SMTP-релей. Без него не разослать коды входа — а других паролей у участников нет.
			</div>
			<div style="display:flex;flex-direction:column;gap:10px;max-width:420px">
				<div style="display:flex;align-items:center;gap:10px">
					<SectionLabel style="margin:0;width:110px">Хост</SectionLabel>
					<Input admin mono style="flex:1" bind:value={host} onchange={() => void persist()} />
				</div>
				<div style="display:flex;align-items:center;gap:10px">
					<SectionLabel style="margin:0;width:110px">Порт</SectionLabel>
					<Input
						admin
						style="width:88px"
						type="number"
						bind:value={port}
						onchange={() => void persist()}
					/>
				</div>
				<div style="display:flex;align-items:center;gap:10px">
					<SectionLabel style="margin:0;width:110px">Логин</SectionLabel>
					<Input admin mono style="flex:1" bind:value={username} onchange={() => void persist()} />
				</div>
				<div style="display:flex;align-items:center;gap:10px">
					<SectionLabel style="margin:0;width:110px">Пароль</SectionLabel>
					<Input
						admin
						style="flex:1"
						type="password"
						placeholder={configured ? 'не менять' : ''}
						bind:value={password}
						onchange={() => void persist()}
					/>
				</div>
				<div style="display:flex;align-items:center;gap:10px">
					<SectionLabel style="margin:0;width:110px">От кого</SectionLabel>
					<Input admin mono style="flex:1" bind:value={from} onchange={() => void persist()} />
				</div>
			</div>
			<SectionLabel style="margin:28px 0 8px">Проверочное письмо</SectionLabel>
			<div style="display:flex;align-items:center;gap:12px;max-width:420px">
				<Input admin style="flex:1" placeholder="куда" bind:value={testTo} />
				<TextButton variant="adminBox" style="font-weight:600" onclick={() => void sendTest()}
					>Отправить</TextButton
				>
			</div>
			{#if saving}
				<Hint style="margin-top:12px">Сохранение…</Hint>
			{:else if notice}
				<Hint style="margin-top:12px">{notice}</Hint>
			{/if}
			{#if error}
				<Hint style="margin-top:12px">{error}</Hint>
			{/if}
		{/if}
	</AdminSection>
</AdminWideLayout>
