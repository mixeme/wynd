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
	import { onDestroy, onMount } from 'svelte';
	import TextButton from '$ui/forms/TextButton.svelte';
	import VoiceWave from '$ui/data/VoiceWave.svelte';
	import { VOICE_MAX_MS, canRecord, formatDuration } from '$lib/media/record';
	import { VoiceRecorder, type VoiceTake } from '$lib/media/voiceRecorder.svelte';

	let {
		placeholder = 'Написать в журнал…',
		value = $bindable(''),
		members = [],
		busy = false,
		onsend,
		oncompose,
		onphotos,
		onvoice,
		onvideo,
		status = '',
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
		// Голосовое и видео (C14, 4.21–4.24): пока поле пустое, круглая
		// кнопка — «Запись», касание — выбор. Без обработчиков — как раньше.
		onvoice?: (take: VoiceTake) => void;
		onvideo?: () => void;
		/** Строка над полосой: «Готовим видео… 40%». */
		status?: string;
		class?: string;
		style?: string;
	} = $props();

	const canSend = $derived(Boolean(value.trim()) && !busy);

	const recorder = new VoiceRecorder();
	onDestroy(() => recorder.dispose());
	// Микрофон браузер даёт только на защищённом адресе; узнаём после загрузки.
	let recordOk = $state(false);
	onMount(() => {
		recordOk = canRecord();
	});
	const recordable = $derived(
		recordOk && Boolean(onvoice || onvideo) && !value.trim() && !busy && recorder.phase === 'idle'
	);
	let menuOpen = $state(false);

	function toggleMenu(e: MouseEvent) {
		e.stopPropagation();
		menuOpen = !menuOpen;
	}

	$effect(() => {
		if (!menuOpen) return;
		const close = () => (menuOpen = false);
		window.addEventListener('click', close);
		return () => window.removeEventListener('click', close);
	});

	function pickVoice() {
		menuOpen = false;
		void recorder.start();
	}

	function pickVideo() {
		menuOpen = false;
		onvideo?.();
	}

	// Проверка перед отправкой (4.24): прослушать записанное.
	let reviewAudio: HTMLAudioElement | undefined = $state();
	let reviewPlaying = $state(false);
	let reviewProgress = $state(0);

	function toggleReview() {
		const el = reviewAudio;
		if (!el) return;
		if (el.paused) void el.play();
		else el.pause();
	}

	// Ход прослушивания — каждый кадр, не по редкому timeupdate. У только что
	// записанного файла браузер часто не знает длительность (Infinity), и
	// прогресс стоял на нуле: длительность берём у таймера записи.
	let reviewFrame = 0;
	function measureReview() {
		const el = reviewAudio;
		if (!el) return;
		const total =
			el.duration && isFinite(el.duration) ? el.duration : recorder.elapsedMs / 1000;
		reviewProgress = total > 0 ? Math.min(1, el.currentTime / total) : 0;
	}
	function trackReview() {
		measureReview();
		if (reviewAudio && !reviewAudio.paused) reviewFrame = requestAnimationFrame(trackReview);
	}
	onDestroy(() => cancelAnimationFrame(reviewFrame));

	function sendVoice() {
		const take = recorder.take;
		if (!take) return;
		reviewAudio?.pause();
		onvoice?.(take);
		recorder.discard();
		reviewProgress = 0;
	}

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
	{#if status}
		<div class="comp-status">{status}</div>
	{/if}
	{#if menuOpen}
		<div class="rec-menu">
			{#if onvoice}
				<button type="button" onclick={pickVoice}><Icon name="mic" />Голосовое</button>
			{/if}
			{#if onvideo}
				<button type="button" onclick={pickVideo}><Icon name="video" />Видео</button>
			{/if}
		</div>
	{/if}
	{#if recorder.phase === 'recording'}
		<div class="f rec">
			<span class="rec-dot"></span>
			<span class="rec-time" class:late={recorder.elapsedMs > VOICE_MAX_MS - 60_000}
				>{formatDuration(recorder.elapsedMs)}</span
			>
			<VoiceWave peaks={recorder.recent} />
			<TextButton onclick={() => recorder.discard()}>Отмена</TextButton>
		</div>
		<button type="button" class="send" aria-label="Стоп" onclick={() => void recorder.stop()}>
			<Icon name="stop" style="color:#fff;width:19px;height:19px" />
		</button>
	{:else if recorder.phase === 'review'}
		<div class="f rec">
			<IconButton
				name={reviewPlaying ? 'pause' : 'play'}
				label={reviewPlaying ? 'Пауза' : 'Прослушать'}
				onclick={toggleReview}
			/>
			<VoiceWave peaks={recorder.peaks} progress={reviewProgress} />
			<span class="rec-time">{formatDuration(recorder.elapsedMs)}</span>
			<IconButton name="trash" label="Удалить запись" onclick={() => recorder.discard()} />
			<audio
				bind:this={reviewAudio}
				src={recorder.url}
				preload="auto"
				onplay={() => {
					reviewPlaying = true;
					cancelAnimationFrame(reviewFrame);
					reviewFrame = requestAnimationFrame(trackReview);
				}}
				onpause={() => (reviewPlaying = false)}
				ontimeupdate={measureReview}
				onended={(e) => {
					reviewPlaying = false;
					cancelAnimationFrame(reviewFrame);
					reviewProgress = 0;
					// Запись браузера — без оглавления: перемотка в начало шла
					// несколько секунд, и повторное прослушивание запаздывало.
					// Открываем файл заново — он играет с начала сразу.
					e.currentTarget.load();
				}}
			></audio>
		</div>
		<button type="button" class="send" aria-label="Отправить голосовое" onclick={sendVoice}>
			<Icon name="send" style="color:#fff;width:19px;height:19px" />
		</button>
	{:else}
	<!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
	<div class="f" class:ink={canSend} onclick={focusField}>
		<TextArea
			variant="comment"
			rows={1}
			placeholder={recorder.error || placeholder}
			bind:el={bodyInput}
			bind:value
			oninput={onInput}
			onkeyup={syncMentionPicker}
		/>
		{#if onphotos}
			<IconButton
				name="photo"
				label="Фото"
				stopPropagation
				style="margin-left:auto"
				onclick={() => photoPicker?.open()}
			/>
		{/if}
		{#if oncompose}
			<IconButton
				name="chevr"
				label="Развернуть"
				stopPropagation
				style="margin-left:{onphotos ? '2px' : 'auto'}"
				onclick={() => oncompose?.()}
			/>
		{/if}
	</div>
	<!-- Вне рамки: нажатие, которым открывается выбор файлов, всплывало до неё,
	     курсор вставал в поле, и телефон сначала показывал клавиатуру. -->
	{#if onphotos}
		<FilePicker
			bind:this={photoPicker}
			accept="image/*,video/*"
			multiple
			onfiles={(files) => onphotos?.(files)}
		/>
	{/if}
	{#if recordable}
		<button
			type="button"
			class="send"
			aria-label="Запись"
			aria-expanded={menuOpen ? 'true' : 'false'}
			onclick={toggleMenu}
		>
			<Icon name="rec" style="color:#fff;width:19px;height:19px" />
		</button>
	{:else}
		<button
			type="button"
			class="send"
			aria-label="Отправить"
			disabled={!canSend}
			onclick={handleSendClick}
		>
			<Icon name="send" style="color:#fff;width:19px;height:19px" />
		</button>
	{/if}
	{/if}
	</div>
</div>

<style>
	.send:disabled {
		cursor: default;
	}
</style>
