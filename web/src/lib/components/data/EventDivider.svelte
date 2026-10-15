<script lang="ts">
	// cutoff — граница архивации в ленте: того же вида, что граница
	// непрочитанного (владелец, 2026-10-15).
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

	const variantClass = $derived(variant === 'event' ? '' : 'new');
</script>

<div class="ev {variantClass} {className}" {style} role={variant === 'cutoff' ? 'separator' : undefined}>
	<span>{text}</span>
</div>
