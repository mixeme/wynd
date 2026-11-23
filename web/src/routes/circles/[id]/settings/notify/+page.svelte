<script lang="ts">
	import { navigateBack } from '$lib/navigation/up';
	import { getContext, onMount } from 'svelte';
	import Chip from '$ui/forms/Chip.svelte';
	import ChipGroup from '$ui/forms/ChipGroup.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Label from '$ui/forms/Label.svelte';
	import Loading from '$ui/Loading.svelte';
	import SettingsRow from '$ui/data/SettingsRow.svelte';
	import Switch from '$ui/forms/Switch.svelte';
	import FormLayout from '$lib/layouts/FormLayout.svelte';
	import { authErrorHint } from '$lib/auth/auth';
	import {
		cacheNotifyPrefs,
		fetchMembers,
		fetchNotifyPrefs,
		loadSettingsBundleCached,
		saveNotifyPrefs,
		type NotifyPrefs
	} from '$lib/circles/settings';
	import {
		baselineFromPrefs,
		muteKeyFromUntil,
		muteUntilFromKey,
		notifyBaselineEqual,
		type MuteKey
	} from '$lib/settings/notify-mute';
	import { CIRCLE_CTX, type CircleContext } from '$lib/journal/context';

	const circle = getContext<CircleContext>(CIRCLE_CTX);

	let posts = $state(true);
	let commentsMine = $state(true);
	let commentsAll = $state(false);
	let reactions = $state(false);
	let events = $state(false);
	let mute = $state<MuteKey>('none');
	let error = $state('');
	let ready = $state(false);
	let soloCircle = $state(false);
	let baseline = $state<ReturnType<typeof baselineFromPrefs> | null>(null);

	async function persist() {
		if (!ready || !baseline) return;
		const next = {
			posts,
			comments_mine: commentsMine,
			comments_all: commentsAll,
			reactions,
			events,
			mute_until: muteUntilFromKey(mute)
		};
		if (notifyBaselineEqual(next, baseline)) return;
		try {
			const prefs = await saveNotifyPrefs(circle.origin, circle.circleId, next);
			applyPrefs(prefs);
			// Снимок семейства настроек — откуда экран берёт первый кадр: пусть
			// не отстаёт от только что сохранённого.
			void cacheNotifyPrefs(circle.origin, circle.circleId, prefs).catch(() => {});
		} catch (err) {
			error = authErrorHint(err);
		}
	}

	/** Переключатели и подпись приглушения — из снимка или из ответа сети. */
	function applyPrefs(prefs: NotifyPrefs) {
		posts = prefs.posts;
		commentsMine = prefs.comments_mine;
		commentsAll = prefs.comments_all;
		reactions = prefs.reactions;
		events = prefs.events;
		mute = muteKeyFromUntil(prefs.mute_until);
		baseline = baselineFromPrefs(prefs);
	}

	$effect(() => {
		void posts;
		void commentsMine;
		void commentsAll;
		void reactions;
		void events;
		void mute;
		void persist();
	});

	onMount(async () => {
		// Снимок семейства настроек — первый кадр целиком (C125): состав круга и
		// prefs уже на устройстве. Иначе строки «как в обычном круге» исчезали
		// через 0,2 с, когда приходил список участников. Нет полного снимка —
		// ждём ответ, но рисуем экран один раз.
		try {
			const cached = await loadSettingsBundleCached(circle.origin, circle.circleId);
			if (cached?.notify) {
				applyPrefs(cached.notify);
				soloCircle = cached.members.filter((m) => m.status === 'active').length === 1;
				ready = true;
			}
		} catch {
			/* снимка нет — ждём сеть */
		}
		try {
			const [prefs, list] = await Promise.all([
				fetchNotifyPrefs(circle.origin, circle.circleId),
				fetchMembers(circle.origin, circle.circleId)
			]);
			applyPrefs(prefs);
			soloCircle = list.filter((m) => m.status === 'active').length === 1;
		} catch (err) {
			if (!ready) error = authErrorHint(err);
		} finally {
			ready = true;
		}
	});
</script>

<FormLayout app color={circle.color} title="Уведомления круга" onback={navigateBack}>
	{#if !ready}
		<Loading />
	{:else}
		<Label>Присылать</Label>
		{#if !soloCircle}
			<SettingsRow class="pt-2" title="Новые записи">
				{#snippet control()}
					<Switch bind:checked={posts} label="Новые записи" />
				{/snippet}
			</SettingsRow>
		{/if}
		<SettingsRow title="Комментарии к моим записям" class={soloCircle ? 'pt-2' : undefined}>
			{#snippet control()}
				<Switch bind:checked={commentsMine} label="Комментарии к моим записям" />
			{/snippet}
		</SettingsRow>
		{#if !soloCircle}
			<SettingsRow title="Все комментарии">
				{#snippet control()}
					<Switch bind:checked={commentsAll} label="Все комментарии" />
				{/snippet}
			</SettingsRow>
			<SettingsRow title="Реакции">
				{#snippet control()}
					<Switch bind:checked={reactions} label="Реакции" />
				{/snippet}
			</SettingsRow>
			<SettingsRow title="Упоминания" subtitle="всегда">
				{#snippet control()}
					<Switch checked={true} disabled label="Упоминания" />
				{/snippet}
			</SettingsRow>
		{/if}
		<SettingsRow title="События круга">
			{#snippet control()}
				<Switch bind:checked={events} label="События круга" />
			{/snippet}
		</SettingsRow>

		<Label class="mt-16">Приглушить</Label>
		<ChipGroup>
			<Chip selected={mute === 'none'} onclick={() => (mute = 'none')}>Нет</Chip>
			<Chip selected={mute === 'tomorrow'} onclick={() => (mute = 'tomorrow')}>До завтра</Chip>
			<Chip selected={mute === 'week'} onclick={() => (mute = 'week')}>На неделю</Chip>
		</ChipGroup>
		{#if !soloCircle}
			<Hint>Упоминание пробивается через приглушение: это адресация, а не шум.</Hint>
		{/if}

		<Hint class="mt-16">В уведомлении нет текста записи — только круг и что произошло.</Hint>
		{#if error}
			<Hint class="gutter">{error}</Hint>
		{/if}
	{/if}
</FormLayout>
