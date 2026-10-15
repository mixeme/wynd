<script lang="ts">
	// cutoff — граница архивации в ленте (владелец, 2026-10-15): сплошная линия,
	// под ней — что будет с записями ниже.
	type DividerVariant = 'event' | 'unread' | 'cutoff';

	let {
		text,
		variant = 'event',
		class: className = '',
		style = ''
	}: {
		text: string;
		variant?: DividerVariant;
		class?: string;
		style?: string;
	} = $props();

	const variantClass = $derived(variant === 'unread' ? 'new' : '');
</script>

{#if variant === 'cutoff'}
	<div class="ev-cutoff {className}" {style} role="separator">
		<div class="ev-cutoff-note">{text}</div>
	</div>
{:else}
	<div class="ev {variantClass} {className}" {style}>
		<span>{text}</span>
	</div>
{/if}
