<script lang="ts">
	import FilePicker from '$ui/forms/FilePicker.svelte';
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
		onphotos,
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
		// Кнопка «Фото» открывает выбор снимков здесь же; без обработчика
		// её нет — иначе она повторяла бы шеврон.
		onphotos?: (files: File[]) => void;
		class?: string;
		style?: string;
	} = $props();

	const canSend = $derived(Boolean(value.trim()) && !busy);

	let bodyInput: HTMLTextAreaElement | undefined = $state();
	let photoPicker: FilePicker | undefined = $state();

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

	const barLines = 3;
	let openedCompose = false;

	function focusField(e: MouseEvent) {
		const target = e.target as HTMLElement | null;
		if (target?.closest('button')) return;
		bodyInput?.focus();
	}

	// fromInput — набор человека. Полный экран открывает только он: замер
	// при монтировании шёл до раскладки (ширина 0), подсказка ломалась по
	// буквам, и лента сама уходила на экран записи.
	function resizeField(fromInput = false) {
		const el = bodyInput;
		if (!el || el.offsetWidth === 0) return;
		el.style.height = 'auto';
		const cs = getComputedStyle(el);
		const line = parseFloat(cs.lineHeight) || 20;
		// Отступ сверху и снизу — место под курсор (ui.css .comp .f .inp);
		// scrollHeight и height (border-box) его включают.
		const pad = (parseFloat(cs.paddingTop) || 0) + (parseFloat(cs.paddingBottom) || 0);
		const cap = line * barLines + pad;
		const full = el.scrollHeight;
		if (full > cap + 1) {
			if (oncompose) {
				el.style.height = `${cap}px`;
				el.style.overflowY = 'hidden';
				if (fromInput && value.trim() && !openedCompose) {
					openedCompose = true;
					// После кадра, чтобы в полный экран ушёл уже дописанный текст.
					queueMicrotask(() => oncompose?.());
				}
				return;
			}
			// У комментария нет экрана записи: после трёх строк поле растёт дальше.
			const room = Math.min(full, line * 8 + pad);
			el.style.height = `${room}px`;
			el.style.overflowY = full > room ? 'auto' : 'hidden';
			return;
		}
		openedCompose = false;
		el.style.overflowY = 'hidden';
		el.style.height = `${Math.max(full, line + pad)}px`;
	}

	$effect(() => {
		void value;
		void bodyInput;
		queueMicrotask(() => resizeField());
	});

	function onInput() {
		syncMentionPicker();
		resizeField(true);
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
	<!-- Нажатие мимо строки в рамке ставит курсор в поле. С клавиатуры
	     поле достаётся Tab напрямую, отдельная роль рамке не нужна. -->
	<!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
	<div class="f" class:ink={canSend} onclick={focusField}>
		<TextArea
			variant="comment"
			rows={1}
			{placeholder}
			bind:el={bodyInput}
			bind:value
			oninput={onInput}
			onkeyup={syncMentionPicker}
		/>
		{#if onphotos}
			<IconButton
				name="photo"
				label="Фото"
				size="sm"
				stopPropagation
				style="margin-left:auto"
				onclick={() => photoPicker?.open()}
			/>
			<FilePicker
				bind:this={photoPicker}
				accept="image/*,video/*"
				multiple
				onfiles={(files) => onphotos?.(files)}
			/>
		{/if}
		{#if oncompose}
			<IconButton
				name="chevr"
				label="Развернуть"
				size="sm"
				stopPropagation
				style="margin-left:{onphotos ? '8px' : 'auto'}"
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
