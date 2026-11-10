<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import Loading from '$ui/Loading.svelte';
	import AdminWideLayout from '$lib/layouts/AdminWideLayout.svelte';
	import { rememberAdminPath } from '$lib/navigation/afterLogin';
	import { reconcileAdminSession } from '$lib/session/session.svelte';

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
		void reconcileAdminSession().then((session) => {
			if (!session) {
				ready = false;
				// Ссылка из письма ведёт на конкретный экран панели: запоминаем
				// его, и после входа возвращает туда, а не на «/admin».
				rememberAdminPath(page.url.pathname + page.url.search + page.url.hash);
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
		<Loading />
	</AdminWideLayout>
{/if}
