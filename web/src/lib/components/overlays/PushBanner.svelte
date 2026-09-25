<script lang="ts">
	let {
		initial,
		color,
		title,
		subtitle,
		time,
		faded = false,
		class: className = '',
		style = ''
	}: {
		initial: string;
		color: string;
		title: string;
		subtitle: string;
		time: string;
		faded?: boolean;
		class?: string;
		style?: string;
	} = $props();
</script>

<div class="push {className}" style:opacity={faded ? 0.55 : undefined} {style}>
	<div class="sq" style:background={color}>{initial}</div>
	<div>
		<div style="font-weight:600">{title}</div>
		<div style="font-size:12.5px;color:var(--muted)">{subtitle}</div>
	</div>
	<span class="tm">{time}</span>
</div>
