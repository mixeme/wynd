<script lang="ts">
	type Segment = {
		width: string;
		color: string;
	};

	let {
		segments,
		threshold,
		class: className = '',
		style = ''
	}: {
		segments: Segment[];
		threshold?: string;
		class?: string;
		style?: string;
	} = $props();
</script>

<div class={className} style="position:relative;{style}">
	<div class="stack">
		{#each segments as segment (segment.color + segment.width)}
			<u style="width:{segment.width};background:{segment.color}"></u>
		{/each}
	</div>
	{#if threshold}
		<div
			style="position:absolute;left:{threshold};top:-14px;bottom:-4px;width:1px;background:var(--terracotta)"
		></div>
		<div
			style="position:absolute;left:{threshold};top:-30px;transform:translateX(-50%);font-size:11.5px;color:var(--terracotta)"
		>
			порог 90%
		</div>
	{/if}
</div>
