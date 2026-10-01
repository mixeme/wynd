<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { WORD, plural } from '$lib/format/plural';
	import { formatAdminDayYear } from '$lib/format/time';
	import AdminSection from '$ui/admin/AdminSection.svelte';
	import DataTable from '$ui/admin/DataTable.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Loading from '$ui/Loading.svelte';
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

	function pluralPeople(n: number): string {
		return plural(n, WORD.person);
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
			<Loading compact />
		{:else if error}
			<Hint>{error}</Hint>
		{:else}
			<div class="flex-mid gap-16 mb-16">
				<h4 class="m-0">Люди</h4>
				<SearchField
					placeholder="почта"
					style="max-width:260px;flex:1"
					bind:value={query}
				/>
				<span class="ml-auto note"
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
							class="pointer"
							onclick={() => goto(`/admin/people/${acc.id}`)}
						>
							<td class="n">{acc.email}</td>
							<td class="muted">{circlesCell(acc)}</td>
							<td class="muted">с {formatAdminDayYear(acc.created_at)}</td>
							<td class="chev"
								><Icon name="chevr" size="sm" /></td
							>
						</tr>
					{/each}
				</tbody>
			</DataTable>
			<div class="fine mt-14 lh-16">
				Панель знает почту и круги, не лица. Строка ведёт в карточку — закрыть вход и удалить
				можно только там.
			</div>
		{/if}
	</AdminSection>
</AdminWideLayout>
