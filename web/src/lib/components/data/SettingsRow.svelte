<script lang="ts">
	import Icon, { type IconName } from '$ui/Icon.svelte';

	let {
		title,
		subtitle,
		value,
		icon,
		link = false,
		chevron = true,
		onclick,
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
		class?: string;
		style?: string;
	} = $props();
</script>

{#if onclick}
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
