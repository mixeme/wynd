<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import Hint from '$ui/forms/Hint.svelte';
	import AdminWideLayout from '$lib/layouts/AdminWideLayout.svelte';
	import { loadAdminSession } from '$lib/session/session.svelte';

	let { children } = $props();

	const publicPath = $derived(
		page.url.pathname === '/admin/bootstrap' || page.url.pathname === '/admin/login'
	);

	let ready = $state(false);

	$effect(() => {
		const isPublic = publicPath;
		if (isPublic) {
			ready = true;
			return;
		}
		void loadAdminSession().then((session) => {
			if (!session) {
				ready = false;
				goto('/admin/login');
				return;
			}
			ready = true;
		});
	});
</script>

{#if publicPath || ready}
	{@render children()}
{:else}
	<AdminWideLayout app nav={false}>
		<Hint>Загрузка…</Hint>
	</AdminWideLayout>
{/if}
