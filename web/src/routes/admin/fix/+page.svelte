<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import AdminSection from '$ui/admin/AdminSection.svelte';
	import Chip from '$ui/forms/Chip.svelte';
	import ChipGroup from '$ui/forms/ChipGroup.svelte';
	import CodeBlock from '$ui/admin/CodeBlock.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import TextButton from '$ui/forms/TextButton.svelte';
	import AdminWideLayout from '$lib/layouts/AdminWideLayout.svelte';
	import { authErrorHint } from '$lib/auth/auth';
	import { fetchProxySnippet, serverCaption } from '$lib/admin/admin';

	const kinds = ['nginx', 'caddy', 'traefik'] as const;
	let kind = $state<(typeof kinds)[number]>('nginx');
	let snippet = $state('');
	let server = $state('');
	let error = $state('');
	let copied = $state(false);

	async function load(next: (typeof kinds)[number]) {
		kind = next;
		copied = false;
		try {
			const data = await fetchProxySnippet(next);
			snippet = data.snippet;
		} catch (err) {
			error = authErrorHint(err);
		}
	}

	async function copy() {
		if (!snippet) return;
		await navigator.clipboard.writeText(snippet);
		copied = true;
	}

	onMount(async () => {
		server = await serverCaption();
		await load('nginx');
	});
</script>

<AdminWideLayout app active="Проверка" {server}>
	<AdminSection>
		<div style="font-size:11.5px;color:var(--faint);margin-bottom:8px">Проверка · Прокси</div>
		<h4 style="margin-bottom:8px">Готовый фрагмент конфига</h4>
		<div style="font-size:12.5px;color:var(--muted);max-width:620px;line-height:1.6">
			Wynd не трогает ваш прокси и не перезапускает его: вставляете вы сами. Обе поломки прокси
			чинятся одной вставкой.
		</div>
		<ChipGroup style="margin:16px 0 12px">
			{#each kinds as item (item)}
				<Chip selected={kind === item} onclick={() => void load(item)}>{item}</Chip>
			{/each}
		</ChipGroup>
		{#if error}
			<Hint>{error}</Hint>
		{:else}
			<CodeBlock>{snippet}</CodeBlock>
		{/if}
		<div style="display:flex;align-items:center;gap:12px;margin-top:16px">
			<TextButton variant="adminBox" style="font-weight:600" onclick={() => void copy()}>
				{copied ? 'Скопировано' : 'Скопировать'}
			</TextButton>
			<TextButton variant="adminBox" onclick={() => goto('/admin/check')}>
				Проверить снова
			</TextButton>
			<span style="font-size:11.5px;color:var(--faint)"
				>Wynd не трогает ваш прокси и не перезапускает его: вставляете вы сами.</span
			>
		</div>
	</AdminSection>
</AdminWideLayout>
