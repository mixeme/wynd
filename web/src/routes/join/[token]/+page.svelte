<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import Button from '$ui/forms/Button.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Input from '$ui/forms/Input.svelte';
	import Label from '$ui/forms/Label.svelte';
	import ServerRow from '$ui/data/ServerRow.svelte';
	import FormLayout from '$lib/layouts/FormLayout.svelte';
	import { authErrorHint, fetchInstance, sendAuthCode } from '$lib/auth/auth';
	import { displayHost } from '$lib/auth/origin';
	import { savePendingAuth } from '$lib/auth/pending';

	let { data } = $props();
	const token = data.token;

	let email = $state('');
	let instanceName = $state('');
	let loading = $state(false);
	let error = $state('');

	onMount(async () => {
		try {
			const info = await fetchInstance('');
			instanceName = info.name;
		} catch {
			error = 'Сервер недоступен';
		}
	});

	async function onSubmit() {
		error = '';
		const trimmed = email.trim();
		if (!trimmed) {
			error = 'Введите почту';
			return;
		}
		loading = true;
		try {
			const info = await fetchInstance('');
			const pending = {
				origin: '',
				email: trimmed,
				flow: 'invite' as const,
				inviteToken: token,
				inviteName: '',
				circleInvite: false,
				instanceName: info.name
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
	const serverSubtitle = $derived(`${displayHost('')} · приглашение на сервер`);
</script>

<FormLayout shell app title="Вас позвали на сервер" onback={() => goto('/')}>
	<ServerRow name={instanceName || '…'} subtitle={serverSubtitle} card />
	<Label style="margin-top:16px">Почта</Label>
	<Input active gray type="email" autocomplete="email" bind:value={email} />
	<Hint>Пришлём код для входа. Пароля нет.</Hint>
	<Button {loading} onclick={onSubmit}>Получить код</Button>
	<Hint>
		Сервер хранит данные незашифрованными. Присоединение к этому серверу означает, что вы
		доверяете его администратору.
	</Hint>
	<Hint centered style="margin-top:26px">
		Приглашение на сервер не ведёт ни в один круг:<br />заведёте свой или подождёте, пока позовут.
	</Hint>
	{#if error}
		<Hint style="margin-top:12px">{error}</Hint>
	{/if}
</FormLayout>
