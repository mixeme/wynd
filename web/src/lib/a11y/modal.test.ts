import { createRawSnippet, flushSync, mount, unmount } from 'svelte';
import { describe, expect, it, vi } from 'vitest';
import OverlayLayout from '$lib/layouts/OverlayLayout.svelte';
import { modal } from './modal';

const body = createRawSnippet(() => ({
	render: () => '<div><button id="one">Один</button><button id="two">Два</button></div>',
	setup: () => () => {}
}));

function tab(shift = false) {
	document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Tab', shiftKey: shift, bubbles: true }));
}

async function settle() {
	flushSync();
	await Promise.resolve();
}

// Инвариант (план 42, UI-1): закрываемый оверлей — модальный диалог: роль и
// aria-modal, фокус переносится внутрь и ходит по кругу, Escape закрывает,
// после закрытия фокус возвращается туда, где был. Незакрываемый (каталог) —
// не трогает ни фокус, ни клавиши.
describe('modal overlay', () => {
	it('traps focus, closes on Escape and restores focus', async () => {
		const opener = document.createElement('button');
		document.body.append(opener);
		opener.focus();
		const target = document.createElement('div');
		document.body.append(target);
		const ondismiss = vi.fn();
		const instance = mount(OverlayLayout, {
			target,
			props: { variant: 'dialog', label: 'Удалить?', ondismiss, children: body }
		});
		await settle();

		const dialog = target.querySelector('[role="dialog"]');
		expect(dialog?.getAttribute('aria-modal')).toBe('true');
		expect(dialog?.getAttribute('aria-label')).toBe('Удалить?');
		const one = target.querySelector<HTMLButtonElement>('#one');
		const two = target.querySelector<HTMLButtonElement>('#two');
		expect(document.activeElement).toBe(one);

		two?.focus();
		tab();
		expect(document.activeElement).toBe(one);
		tab(true);
		expect(document.activeElement).toBe(two);

		opener.focus();
		expect(document.activeElement).toBe(one);

		document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
		expect(ondismiss).toHaveBeenCalledOnce();

		unmount(instance);
		expect(document.activeElement).toBe(opener);
		opener.remove();
		target.remove();
	});

	// Кадр аватара открывается с фокусом на «Готово» и туда же возвращает
	// фокус, ушедший наружу.
	it('focuses the last element with initialFocus last', async () => {
		const outside = document.createElement('button');
		document.body.append(outside);
		const node = document.createElement('div');
		node.innerHTML = '<button id="cancel">Отмена</button><button id="done">Готово</button>';
		document.body.append(node);
		const action = modal(node, { ondismiss: () => {}, initialFocus: 'last' });
		await settle();
		const done = node.querySelector<HTMLButtonElement>('#done');
		expect(document.activeElement).toBe(done);
		node.querySelector<HTMLButtonElement>('#cancel')?.focus();
		outside.focus();
		expect(document.activeElement).toBe(done);
		action.destroy?.();
		outside.remove();
		node.remove();
	});

	it('leaves a static overlay without ondismiss alone', async () => {
		const outside = document.createElement('button');
		document.body.append(outside);
		outside.focus();
		const target = document.createElement('div');
		document.body.append(target);
		const instance = mount(OverlayLayout, { target, props: { children: body } });
		await settle();
		expect(target.querySelector('[role="dialog"]')).toBeNull();
		expect(document.activeElement).toBe(outside);
		unmount(instance);
		outside.remove();
		target.remove();
	});
});
