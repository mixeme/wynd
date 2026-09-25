<script lang="ts">
	type DividerVariant = 'event' | 'unread';

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

<div class="ev {variantClass} {className}" {style}>
	<span>{text}</span>
</div>
