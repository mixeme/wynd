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
		<div style="font-size:11.5px;color:var(--faint);margin-bottom:8px">Проверка · Прокси</div>
		<h4 style="margin-bottom:8px">{fixText.title}</h4>
		<div style="font-size:12.5px;color:var(--muted);max-width:620px;line-height:1.6">{fixText.body}</div>
		<div
			style="display:flex;flex-wrap:wrap;gap:10px;margin:16px 0 18px;font-size:11.5px;color:var(--muted)"
		>
			<span class="inp mono">X-Forwarded-For: {xff}</span>
			<span class="inp mono">X-Real-IP: {xri}</span>
			<span class="inp mono">клиент: {clientIp}</span>
		</div>
		<ChipGroup style="margin:0 0 12px">
			{#each kinds as item (item)}
				<Chip selected={kind === item} onclick={() => void load(item)}>{item}</Chip>
			{/each}
		</ChipGroup>
		{#if error}
			<Hint>{error}</Hint>
		{:else}
			<CodeBlock {lines} />
		{/if}
		<div style="display:flex;align-items:center;gap:12px;margin-top:16px">
			<TextButton variant="adminBox" style="font-weight:600" onclick={() => void copySnippet()}>
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
