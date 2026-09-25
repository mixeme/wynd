<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import AdminSection from '$ui/admin/AdminSection.svelte';
	import Chip from '$ui/forms/Chip.svelte';
	import ChipGroup from '$ui/forms/ChipGroup.svelte';
	import CodeBlock from '$ui/admin/CodeBlock.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import TextButton from '$ui/forms/TextButton.svelte';
	import AdminWideLayout from '$lib/layouts/AdminWideLayout.svelte';
	import { authErrorHint } from '$lib/auth/auth';
	import { getAdminSession } from '$lib/idb/db';
	import { fetchProbeDiagnostics } from '$lib/admin/external-probe';
	import { fixCopy } from '$lib/admin/fix-copy';
	import { annotateProxySnippet } from '$lib/admin/proxy-snippet';
	import { fetchProxySnippet, serverCaption } from '$lib/admin/admin';

	const kinds = ['nginx', 'caddy', 'traefik'] as const;
	let kind = $state<(typeof kinds)[number]>('nginx');
	let snippet = $state('');
	let server = $state('');
	let error = $state('');
	let copied = $state(false);
	let xff = $state('—');
	let xri = $state('—');
	let clientIp = $state('—');

	const fail = $derived($page.url.searchParams.get('fail') ?? '');
	const fixText = $derived(fixCopy(fail));
	const lines = $derived(annotateProxySnippet(snippet, fail));

	function chipValue(value: string): string {
		return value.trim() === '' ? '—' : value;
	}

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

	async function copySnippet() {
		if (!lines.length) return;
		await navigator.clipboard.writeText(lines.map((line) => line.text).join('\n'));
		copied = true;
	}

	onMount(async () => {
		server = await serverCaption();
		await load('nginx');
		const origin = (await getAdminSession())?.origin ?? '';
		if (origin) {
			try {
				const probe = await fetchProbeDiagnostics(origin);
				if (probe) {
					xff = chipValue(probe.x_forwarded_for ?? '');
					xri = chipValue(probe.x_real_ip ?? '');
					clientIp = chipValue(probe.client_ip ?? '');
				}
			} catch {
				/* probe optional on fix screen */
			}
		}
	});
</script>

<AdminWideLayout app active="Проверка" {server}>
	<AdminSection>
		<div class="sz-11 faint mb-8">Проверка · Прокси</div>
		<h4 style="margin-bottom:8px">{fixText.title}</h4>
		<div class="note lh-16" style="max-width:620px">{fixText.body}</div>
		<div class="flex wrap gap-10 mt-16 mb-18 sz-11 muted">
			<span class="inp mono">X-Forwarded-For: {xff}</span>
			<span class="inp mono">X-Real-IP: {xri}</span>
			<span class="inp mono">клиент: {clientIp}</span>
		</div>
		<ChipGroup class="mx-0 mb-12">
			{#each kinds as item (item)}
				<Chip selected={kind === item} onclick={() => void load(item)}>{item}</Chip>
			{/each}
		</ChipGroup>
		{#if error}
			<Hint>{error}</Hint>
		{:else}
			<CodeBlock {lines} />
		{/if}
		<div class="flex-mid gap-12 mt-16">
			<TextButton variant="adminBox" class="bold" onclick={() => void copySnippet()}>
				{copied ? 'Скопировано' : 'Скопировать'}
			</TextButton>
			<TextButton variant="adminBox" onclick={() => goto('/admin/check')}>
				Проверить снова
			</TextButton>
			<span class="sz-11 faint"
				>Wynd не трогает ваш прокси и не перезапускает его: вставляете вы сами.</span
			>
		</div>
	</AdminSection>
</AdminWideLayout>
