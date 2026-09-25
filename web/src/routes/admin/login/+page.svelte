<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import Hint from '$ui/forms/Hint.svelte';
	import Input from '$ui/forms/Input.svelte';
	import Label from '$ui/forms/Label.svelte';
	import Button from '$ui/forms/Button.svelte';
	import AdminSection from '$ui/admin/AdminSection.svelte';
	import AdminWideLayout from '$lib/layouts/AdminWideLayout.svelte';
	import { authErrorHint, loginAdmin } from '$lib/auth/auth';
	import { reconcileAdminSession, storeAdminSession } from '$lib/session/session.svelte';

	let password = $state('');
	let loading = $state(false);
	let error = $state('');

	onMount(async () => {
		const session = await reconcileAdminSession();
		if (session) goto('/admin');
	});

	async function onSubmit() {
		error = '';
		if (!password) {
			error = 'Введите пароль администратора';
			return;
		}
		loading = true;
		try {
			const session = await loginAdmin(password);
			await storeAdminSession({ origin: '', token: session.token });
			goto('/admin');
		} catch (err) {
			error = authErrorHint(err);
		} finally {
			loading = false;
		}
	}
</script>

<AdminWideLayout app nav={false}>
	<AdminSection title="Панель администратора">
		<Hint style="margin-bottom:18px">
			Отдельный пароль, не тот, которым входят в круги. Журналов кругов здесь нет.
		</Hint>
		<Label>Пароль</Label>
		<Input
			active
			type="password"
			autocomplete="current-password"
			bind:value={password}
			onkeydown={(e) => e.key === 'Enter' && onSubmit()}
		/>
		<Button
			style="margin-top:20px"
			loading={loading}
			disabled={loading}
			onclick={loading ? () => {} : () => void onSubmit()}
		>
			Войти
		</Button>
		{#if error}
			<Hint style="margin-top:12px">{error}</Hint>
		{/if}
	</AdminSection>
</AdminWideLayout>
