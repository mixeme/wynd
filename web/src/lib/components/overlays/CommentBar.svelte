<script lang="ts">
	import MemberRow from '$ui/data/MemberRow.svelte';
	import MentionPicker from '$ui/forms/MentionPicker.svelte';
	import Icon from '$ui/Icon.svelte';
	import IconButton from '$ui/forms/IconButton.svelte';
	import TextArea from '$ui/forms/TextArea.svelte';
	import { memberAvatarColor } from '$lib/auth/invites';
	import { circleInitial } from '$lib/circles/meta';
	import { filterMembersByMention, insertMention, mentionQueryAt } from '$lib/journal/mentions';

	let {
		placeholder = 'Написать в журнал…',
		value = $bindable(''),
		members = [],
		busy = false,
		onsend,
		oncompose,
		class: className = '',
		style = ''
	}: {
		placeholder?: string;
		value?: string;
		// Ключ — лицо в круге: учётка соседа сюда не приходит.
		members?: { identity_id: string; name: string }[];
		// Идёт отправка: кнопка гаснет, второе нажатие не создаёт вторую запись (GUI-8).
		busy?: boolean;
		onsend?: () => void;
		oncompose?: () => void;
		class?: string;
		style?: string;
	} = $props();

	const canSend = $derived(Boolean(value.trim()) && !busy);

	let bodyInput: HTMLTextAreaElement | undefined = $state();
	let mentionStart = $state<number | null>(null);
	let mentionQuery = $state('');

	const mentionCandidates = $derived(
		mentionStart == null ? [] : filterMembersByMention(members, mentionQuery)
	);
	const showMentionPicker = $derived(
		members.length > 0 && mentionStart != null && mentionCandidates.length > 0
	);

	function syncMentionPicker() {
		if (!bodyInput || !members.length) {
			mentionStart = null;
			mentionQuery = '';
			return;
		}
		const state = mentionQueryAt(value, bodyInput.selectionStart ?? value.length);
		if (!state) {
			mentionStart = null;
			mentionQuery = '';
			return;
		}
		mentionStart = state.start;
		mentionQuery = state.query;
	}

	function onInput() {
		syncMentionPicker();
	}

	function pickMember(member: { identity_id: string; name: string }) {
		if (mentionStart == null || !bodyInput) return;
		const cursor = bodyInput.selectionStart ?? value.length;
		value = insertMention(value, mentionStart, cursor, member.name);
		const nextPos = mentionStart + member.name.length + 1;
		mentionStart = null;
		mentionQuery = '';
		queueMicrotask(() => {
			bodyInput?.focus();
			bodyInput?.setSelectionRange(nextPos, nextPos);
		});
	}

	function handleSendClick(e: MouseEvent) {
		e.stopPropagation();
		if (canSend) onsend?.();
	}
</script>

<div class="comp-wrap {className}" {style}>
	{#if showMentionPicker}
		<MentionPicker>
			{#each mentionCandidates as member, i (member.identity_id)}
				<MemberRow
					initial={circleInitial(member.name)}
					name={member.name}
					color={memberAvatarColor(i)}
					onclick={() => pickMember(member)}
					style={i === 0 ? 'padding:10px 14px' : undefined}
				/>
			{/each}
		</MentionPicker>
	{/if}
	<div class="comp">
	<div class="f" class:ink={canSend}>
		<TextArea
			variant="comment"
			rows={1}
			{placeholder}
			bind:el={bodyInput}
			bind:value
			oninput={onInput}
			onkeyup={syncMentionPicker}
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
</div>

<style>
	.send:disabled {
		cursor: default;
	}
</style>
