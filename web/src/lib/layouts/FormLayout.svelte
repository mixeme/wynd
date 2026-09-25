<script lang="ts">
	import BackBar from '$ui/chrome/BackBar.svelte';
	import PhoneFrame from '$ui/chrome/PhoneFrame.svelte';
	import StatusBar from '$ui/chrome/StatusBar.svelte';
	import TextButton from '$ui/forms/TextButton.svelte';
	import type { CircleColor } from '$lib/theme/colors';
	import type { Snippet } from 'svelte';

	let {
		color,
		shell = false,
		dark = false,
		app = false,
		height,
		class: className = '',
		title,
		right,
		search,
		compact = false,
		bar,
		compose = false,
		publishLabel = 'Опубликовать',
		canPublish = false,
		publishing = false,
		oncancel,
		onpublish,
		footer,
		onback,
		children
	}: {
		color?: CircleColor;
		shell?: boolean;
		dark?: boolean;
		app?: boolean;
		height?: string;
		class?: string;
		title?: string;
		right?: string;
		search?: string;
		compact?: boolean;
		bar?: Snippet;
		compose?: boolean;
		publishLabel?: string;
		canPublish?: boolean;
		publishing?: boolean;
		oncancel?: () => void;
		onpublish?: () => void;
		footer?: Snippet;
		onback?: () => void;
		children: Snippet;
	} = $props();
</script>

<PhoneFrame {color} {shell} {dark} {app} {height} class={className}>
	{#if !app}
		<StatusBar />
	{/if}
	{#if compose}
		<div class="cbar">
			<div class="top compose-top">
				<TextButton variant="admin" onclick={() => oncancel?.()}>Отмена</TextButton>
				<span class="t">{title}</span>
				<TextButton
					variant="barAction"
					active
					disabled={!canPublish}
					loading={publishing}
					onclick={() => onpublish?.()}
				>
					{publishLabel}
				</TextButton>
			</div>
			<div style="height:12px"></div>
		</div>
	{:else if bar}
		<BackBar {compact} {onback}>{@render bar()}</BackBar>
	{:else}
		<BackBar {title} {right} {search} {compact} {onback} />
	{/if}
	{#if app && !compose}
		<div class="form-body">
			{@render children()}
		</div>
	{:else}
		{@render children()}
	{/if}
	{#if footer}
		{@render footer()}
	{/if}
</PhoneFrame>
