<script lang="ts">
	import Icon, { type IconName } from '$ui/Icon.svelte';
	import Row from '$ui/data/Row.svelte';
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
		divided,
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
		/** Черта над строкой, под ней или с обеих сторон — отдельная строка
		 *  среди текста (план 47, 2.22). */
		divided?: 'top' | 'bottom' | 'both';
		class?: string;
		style?: string;
	} = $props();

	const rowClass = $derived(
		[
			className,
			divided === 'top' || divided === 'both' ? 'divided-top' : '',
			divided === 'bottom' || divided === 'both' ? 'divided-bottom' : ''
		]
			.filter(Boolean)
			.join(' ')
	);
</script>

<!-- С control (переключатель и т. п.) строка не нажимается целиком — иначе
     вложенная кнопка; заголовок тогда без жирного, как в кадрах. -->
<Row
	onclick={control ? undefined : onclick}
	{link}
	class={rowClass}
	{style}
>
	{#snippet leading()}
		{#if icon}
			<Icon name={icon} />
		{/if}
	{/snippet}
	{#snippet main()}
		{#if link}
			{title}
		{:else if control}
			{#if subtitle}
				<div>{title}</div>
				<div class="sub">{subtitle}</div>
			{:else}
				{title}
			{/if}
		{:else}
			<div style="font-weight:600">{title}</div>
			{#if subtitle}
				<div class="sub">{subtitle}</div>
			{/if}
		{/if}
	{/snippet}
	{#snippet trailing()}
		{#if control}
			{@render control()}
		{:else}
			{#if value}
				<span class="val">{value}</span>
			{/if}
			{#if chevron}
				<Icon name="chevr" size="sm" />
			{/if}
		{/if}
	{/snippet}
</Row>
