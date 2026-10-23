<script lang="ts">
	// cutoff — граница архивации в ленте: того же вида, что граница
	// непрочитанного (владелец, 2026-10-15).
	type DividerVariant = 'event' | 'unread' | 'cutoff';

	let {
		text,
		time,
		variant = 'event',
		class: className = '',
		style = ''
	}: {
		text: string;
		/** Когда случилось — второй строкой, мельче (владелец, 2026-10-23). */
		time?: string;
		variant?: DividerVariant;
		class?: string;
		style?: string;
	} = $props();

	const variantClass = $derived(variant === 'event' ? '' : 'new');
</script>

<div class="ev {variantClass} {className}" {style} role={variant === 'cutoff' ? 'separator' : undefined}>
	<span>{text}{#if time}<small class="ev-t">{time}</small>{/if}</span>
</div>
