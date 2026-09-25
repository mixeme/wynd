<script lang="ts">
	import Icon from '$ui/Icon.svelte';
	import IconButton from '$ui/forms/IconButton.svelte';
	import TextArea from '$ui/forms/TextArea.svelte';

	let {
		placeholder = 'Написать в журнал…',
		value = $bindable(''),
		onsend,
		oncompose,
		class: className = '',
		style = ''
	}: {
		placeholder?: string;
		value?: string;
		onsend?: () => void;
		oncompose?: () => void;
		class?: string;
		style?: string;
	} = $props();

	const canSend = $derived(Boolean(value.trim()));
	const isEmpty = $derived(!value.trim());

	function prefersInlineCompose() {
		return (
			typeof window !== 'undefined' &&
			window.matchMedia('(hover: hover) and (pointer: fine)').matches
		);
	}

	function handleFieldClick() {
		if (isEmpty && !prefersInlineCompose()) oncompose?.();
	}

	function handleSendClick(e: MouseEvent) {
		e.stopPropagation();
		if (canSend) onsend?.();
	}
</script>

<div class="comp {className}" {style}>
	<div class="f" class:ink={canSend}>
		<TextArea
			variant="comment"
			rows={1}
			{placeholder}
			bind:value
			onclick={handleFieldClick}
		/>
		{#if oncompose}
			<IconButton
				name="photo"
				label="Фото"
				size="sm"
				stopPropagation
				style="margin-left:auto"
				onclick={() => oncompose?.()}
			/>
			<IconButton
				name="chevr"
				label="Развернуть"
				size="sm"
				stopPropagation
				style="margin-left:8px"
				onclick={() => oncompose?.()}
			/>
		{/if}
	</div>
	<button
		type="button"
		class="send"
		aria-label="Отправить"
		disabled={!canSend}
		onclick={handleSendClick}
	>
		<Icon name="send" style="color:#fff;width:19px;height:19px" />
	</button>
</div>

<style>
	.send:disabled {
		cursor: default;
	}
</style>
