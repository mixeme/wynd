<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import Hint from '$ui/forms/Hint.svelte';
	import QrScanner from '$ui/forms/QrScanner.svelte';
	import FormLayout from '$lib/layouts/FormLayout.svelte';
	import { interfaceAvailable, interfaceInvite, saveScannedInvite } from '$lib/android/interface';
	let error = $state('');
	let notice = $state('');
	let supported = $state(false);
	const back = () => {
		void goto('/settings/interface', { replaceState: true });
	};
	function onread(text: string): boolean {
		if (!interfaceInvite(text)) {
			notice = 'Это не ссылка-приглашение Wynd';
			return false;
		}
		saveScannedInvite(text);
		back();
		return true;
	}
	onMount(() => {
		if (!interfaceAvailable()) {
			void goto('/settings', { replaceState: true });
			return;
		}
		supported = true;
	});
</script>

<FormLayout shell app title="Сканер" onback={back}>
	{#if supported}<QrScanner {onread} onerror={(message) => (error = message)} />{/if}
	<Hint>{error || notice || 'Наведите камеру на QR-код приглашения — он откроется сам.'}</Hint>
</FormLayout>
