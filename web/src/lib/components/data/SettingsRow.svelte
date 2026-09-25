<script lang="ts">
	import Icon, { type IconName } from '$ui/Icon.svelte';
	import type { Snippet } from 'svelte';

	let {
		title,
		subtitle,
		value,
		icon,
		link = false,
		chevron = true,
		onclick,
		control,
		class: className = '',
		style = ''
	}: {
		title: string;
		subtitle?: string;
		value?: string;
		icon?: IconName;
		link?: boolean;
		chevron?: boolean;
		onclick?: () => void;
		control?: Snippet;
		class?: string;
		style?: string;
	} = $props();
</script>

{#if control}
	<div class="row2 {className}" {style}>
		{#if icon}
			<Icon name={icon} />
		{/if}
		<div class="g" class:link>
			{#if link}
				{title}
			{:else if subtitle}
				<div>{title}</div>
				<div class="sub">{subtitle}</div>
			{:else}
				{title}
			{/if}
		</div>
		{@render control()}
	</div>
{:else if onclick}
	<button type="button" class="row2 {className}" {style} {onclick}>
		{#if icon}
			<Icon name={icon} />
		{/if}
		<div class="g" class:link>
			{#if link}
				{title}
			{:else}
				<div style="font-weight:600">{title}</div>
				{#if subtitle}
					<div class="sub">{subtitle}</div>
				{/if}
			{/if}
		</div>
		{#if value}
			<span class="val">{value}</span>
		{/if}
		{#if chevron}
			<Icon name="chevr" size="sm" />
		{/if}
	</button>
{:else}
	<div class="row2 {className}" {style}>
		{#if icon}
			<Icon name={icon} />
		{/if}
		<div class="g" class:link>
			{#if link}
				{title}
			{:else}
				<div style="font-weight:600">{title}</div>
				{#if subtitle}
					<div class="sub">{subtitle}</div>
				{/if}
			{/if}
		</div>
		{#if value}
			<span class="val">{value}</span>
		{/if}
		{#if chevron}
			<Icon name="chevr" size="sm" />
		{/if}
	</div>
{/if}
