<script lang="ts">
	import { onMount } from 'svelte';
	import AdminNav, { type AdminNavItem } from '$ui/admin/AdminNav.svelte';
	import Mark from '$ui/Mark.svelte';
	import { fetchAdminPending } from '$lib/admin/admin';

	let {
		active = 'Хранилище',
		server,
		nav = true,
		links = false
	}: {
		active?: AdminNavItem;
		server?: string;
		nav?: boolean;
		links?: boolean;
	} = $props();

	// Что ждёт решения администратора — числом у раздела (2026-10-21): видно и
	// без почты администратора.
	let counts = $state<Partial<Record<AdminNavItem, number>>>({});
	onMount(async () => {
		if (!links || !nav) return;
		try {
			const pending = await fetchAdminPending();
			counts = { Хранилище: pending.quota_requests, Оплата: pending.pay_requests };
		} catch {
			/* счётчик необязателен: без него панель та же */
		}
	});
</script>

<div class="admbar">
	<Mark />
	{#if nav}
		<AdminNav {active} {links} {counts} />
	{/if}
	{#if server}
		<span style="margin-left:auto;font-size:12.5px;color:var(--faint)">{server}</span>
	{/if}
</div>
