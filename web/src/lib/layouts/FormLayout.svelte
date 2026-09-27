<script lang="ts">
	import BackBar from '$ui/chrome/BackBar.svelte';
	import PhoneFrame from '$ui/chrome/PhoneFrame.svelte';
	import StatusBar from '$ui/chrome/StatusBar.svelte';
	import IconButton from '$ui/forms/IconButton.svelte';
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
		circleTitle,
		subtitle,
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
		onright,
		children
	}: {
		color?: CircleColor;
		shell?: boolean;
		dark?: boolean;
		app?: boolean;
		height?: string;
		class?: string;
		title?: string;
		circleTitle?: string;
		subtitle?: string;
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
		onright?: () => void;
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
	{:else if circleTitle}
		<div class="cbar">
			<div class="top" style="justify-content:center">
				<span class="t">{circleTitle}</span>
			</div>
			<div style="height:10px"></div>
		</div>
	{:else if bar}
		<BackBar {compact} {onback}>{@render bar()}</BackBar>
	{:else if color && subtitle}
		<div class="cbar">
			<div class="top">
				{#if onback}
					<IconButton name="back" label="Назад" onclick={() => onback()} />
				{/if}
				<span class="t">{title}</span>
				{#if right}
					<span class="rt" style="margin-left:auto;font-size:12.5px;color:rgba(255,255,255,.85)"
						>{right}</span
					>
				{/if}
			</div>
			<div style="padding-bottom:12px;font-size:12.5px;color:rgba(255,255,255,.85)">{subtitle}</div>
		</div>
	{:else}
		<BackBar {title} {right} {search} {compact} {onback} {onright} />
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
