<script lang="ts">
	import Icon from '$ui/Icon.svelte';
	import IconButton from '$ui/forms/IconButton.svelte';
	import { pluralDays } from '$lib/pay/pay';

	type Donate = {
		variant: 'donate';
		text: string;
		dismissible?: boolean;
		onclick: () => void;
		ondismiss?: () => void;
	};

	type Reminder = {
		variant: 'reminder';
		expiresAtLabel: string;
		reminderDaysLeft?: number | null;
		onclick: () => void;
	};

	type Pending = {
		variant: 'pending';
		pendingAtLabel: string;
		expiresAtLabel?: string | null;
	};

	let props: Donate | Reminder | Pending = $props();
</script>

{#if props.variant === 'donate'}
	<div class="pay-banner">
		<button type="button" class="pay-banner-main" onclick={() => props.onclick()}>
			<div class="pay-banner-copy">{props.text}</div>
			<Icon name="chevr" size="sm" class="pay-banner-chev" />
		</button>
		{#if props.dismissible}
			<IconButton
				name="x"
				label="Скрыть"
				style="flex-shrink:0"
				onclick={() => props.ondismiss?.()}
			/>
		{/if}
	</div>
{:else if props.variant === 'reminder'}
	<button type="button" class="pay-reminder" onclick={() => props.onclick()}>
		<div class="pay-banner-copy">
			<div class="pay-banner-strong">Подписка до {props.expiresAtLabel}</div>
			{#if props.reminderDaysLeft != null}
				<div class="pay-banner-muted">
					Через {pluralDays(props.reminderDaysLeft)} круги закроются.
				</div>
			{/if}
		</div>
		<Icon name="chevr" size="sm" class="pay-banner-chev" />
	</button>
{:else}
	<div class="pay-banner pay-pending">
		<div class="pay-banner-strong pay-banner-copy">
			Заявку отправили {props.pendingAtLabel}
		</div>
		<div class="pay-banner-muted pay-banner-copy">
			{#if props.expiresAtLabel}
				Круги открыты до {props.expiresAtLabel}. Пока администратор не ответит, новую заявку
				отправить нельзя.
			{:else}
				Пока администратор не ответит, новую заявку отправить нельзя.
			{/if}
		</div>
	</div>
{/if}
