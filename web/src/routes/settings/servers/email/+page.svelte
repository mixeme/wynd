<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { goUp } from '$lib/navigation/up';
	import Button from '$ui/forms/Button.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Input from '$ui/forms/Input.svelte';
	import SectionLabel from '$ui/data/SectionLabel.svelte';
	import SettingsRow from '$ui/data/SettingsRow.svelte';
	import FormLayout from '$lib/layouts/FormLayout.svelte';
	import { fetchInstance } from '$lib/auth/auth';
	import { INVALID_EMAIL_HINT, isValidParticipantEmail } from '$lib/auth/email';
	import {
		clearPendingEmailChange,
		emailChangeErrorHint,
		loadPendingEmailChange,
		requestEmailChange,
		sameEmail,
		savePendingEmailChange,
		type PendingEmailChange
	} from '$lib/auth/emailChange';

	let pending = $state<PendingEmailChange | undefined>();
	let email = $state('');
	let loading = $state(false);
	let error = $state('');

	onMount(() => {
		pending = loadPendingEmailChange();
		if (!pending) {
			goto('/settings/servers');
			return;
		}
		email = pending.email;
	});

	function back() {
		clearPendingEmailChange();
		void goUp('/settings/servers');
	}

	async function onSubmit() {
		if (!pending) return;
		error = '';
		const trimmed = email.trim();
		if (!trimmed) {
			error = 'Введите почту';
			return;
		}
		if (!isValidParticipantEmail(trimmed)) {
			error = INVALID_EMAIL_HINT;
			return;
		}
		if (sameEmail(trimmed, pending.currentEmail)) {
			error = 'Это ваша нынешняя почта';
			return;
		}
		loading = true;
		try {
			await requestEmailChange(pending.origin, trimmed);
			let codeDelivery = pending.codeDelivery;
			try {
				codeDelivery = (await fetchInstance(pending.origin)).code_delivery ?? 'mail';
			} catch {
				/* письмо ушло; чем его доставили — только подпись на экране кода */
			}
			savePendingEmailChange({
				...pending,
				email: trimmed,
				codeDelivery,
				codeSentAt: Date.now(),
				retryUntil: undefined
			});
			goto('/settings/servers/email/code');
		} catch (err) {
			error = emailChangeErrorHint(err, 'email');
		} finally {
			loading = false;
		}
	}
</script>

<FormLayout shell app title="Новая почта" onback={back}>
	{#if pending}
		<SectionLabel>Сейчас</SectionLabel>
		<SettingsRow
			class="pt-2"
			title={pending.currentEmail}
			subtitle={pending.serverName ? `${pending.serverName} · ${pending.host}` : pending.host}
			chevron={false}
		/>
		<SectionLabel>Новая почта</SectionLabel>
		<Input active type="email" autocomplete="email" bind:value={email} />
		{#if error}
			<Hint>{error}</Hint>
		{/if}
		<Hint>
			Пришлём на неё код. Круги, имена и записи остаются вашими: меняется только адрес, по
			которому вы входите на этот сервер.
		</Hint>
		<Button {loading} onclick={onSubmit}>Получить код</Button>
		<Hint class="mt-18">На прежний адрес уйдёт письмо о том, что почту сменили.</Hint>
	{/if}
</FormLayout>
