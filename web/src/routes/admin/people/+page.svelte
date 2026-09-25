<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import AdminSection from '$ui/admin/AdminSection.svelte';
	import DataTable from '$ui/admin/DataTable.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Icon from '$ui/Icon.svelte';
	import SearchField from '$ui/forms/SearchField.svelte';
	import AdminWideLayout from '$lib/layouts/AdminWideLayout.svelte';
	import { authErrorHint } from '$lib/auth/auth';
	import { fetchAccounts, serverCaption, type AdminAccount } from '$lib/admin/admin';

	let accounts = $state<AdminAccount[]>([]);
	let query = $state('');
	let server = $state('');
	let error = $state('');
	let loading = $state(true);

	const filtered = $derived(
		accounts.filter((acc) => {
			const q = query.trim().toLowerCase();
			if (!q) return true;
			return acc.email.toLowerCase().includes(q);
		})
	);

	function circlesCell(acc: AdminAccount): string {
		if (acc.blocked) {
			return acc.circle_count === 0 ? 'без кругов · вход закрыт' : `${acc.circle_count} · вход закрыт`;
		}
		if (acc.circle_count === 0) return 'без кругов';
		return String(acc.circle_count);
	}

	function formatSince(iso: string): string {
		const d = new Date(iso);
		if (Number.isNaN(d.getTime())) return iso;
		return new Intl.DateTimeFormat('ru-RU', {
			day: 'numeric',
			month: 'long',
			year: 'numeric'
		}).format(d);
	}

	function pluralPeople(n: number): string {
		const mod10 = n % 10;
		const mod100 = n % 100;
		if (mod10 === 1 && mod100 !== 11) return `${n} человек`;
		if (mod10 >= 2 && mod10 <= 4 && (mod100 < 10 || mod100 >= 20)) return `${n} человека`;
		return `${n} человек`;
	}

	onMount(async () => {
		try {
			const [list, caption] = await Promise.all([fetchAccounts(), serverCaption()]);
			accounts = list;
			server = caption;
		} catch (err) {
			error = authErrorHint(err);
		} finally {
			loading = false;
		}
	});
</script>

<AdminWideLayout app active="Люди" {server}>
	<AdminSection>
		{#if loading}
			<Hint>Загрузка…</Hint>
		{:else if error}
			<Hint>{error}</Hint>
		{:else}
			<div style="display:flex;align-items:center;gap:16px;margin-bottom:16px">
				<h4 style="margin:0">Люди</h4>
				<SearchField
					placeholder="почта"
					style="max-width:260px;flex:1"
					bind:value={query}
				/>
				<span style="margin-left:auto;font-size:12.5px;color:var(--muted)"
					>{pluralPeople(accounts.length)}</span
				>
			</div>
			<DataTable>
				<thead>
					<tr>
						<th>Почта</th>
						<th>Круги</th>
						<th>На сервере</th>
						<th></th>
					</tr>
				</thead>
				<tbody>
					{#each filtered as acc (acc.id)}
						<tr
							style:color={acc.blocked ? 'var(--faint)' : undefined}
							style="cursor:pointer"
							onclick={() => goto(`/admin/people/${acc.id}`)}
						>
							<td class="n">{acc.email}</td>
							<td style="color:var(--muted)">{circlesCell(acc)}</td>
							<td style="color:var(--muted)">с {formatSince(acc.created_at)}</td>
							<td style="text-align:right;width:22px;padding-right:0"
								><Icon name="chevr" size="sm" /></td
							>
						</tr>
					{/each}
				</tbody>
			</DataTable>
			<div style="font-size:11.5px;color:var(--faint);margin-top:14px;line-height:1.6">
				Панель знает почту и круги, не лица. Строка ведёт в карточку — закрыть вход и удалить
				можно только там.
			</div>
		{/if}
	</AdminSection>
</AdminWideLayout>
