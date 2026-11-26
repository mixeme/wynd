import { flushSync, mount, unmount } from 'svelte';
import { describe, expect, it } from 'vitest';
import ComposeHeader from './ComposeHeader.svelte';

function render(props: Record<string, unknown> = {}): HTMLElement {
	const target = document.createElement('div');
	const instance = mount(ComposeHeader, {
		target,
		props: { title: 'Кадр', publishLabel: 'Готово', ...props }
	});
	flushSync();
	const copy = target.cloneNode(true) as HTMLElement;
	unmount(instance);
	return copy;
}

describe('ComposeHeader', () => {
	// Правило C98: однострочная шапка — 48 px (4 + 40 + 4), без нижней распорки.
	it('компактная полоса — одна строка без запаса снизу', () => {
		const bar = render().querySelector('.cbar');
		expect(bar).not.toBeNull();
		expect(bar?.querySelectorAll(':scope > div')).toHaveLength(1);
		expect(bar?.querySelector('.top.compose-top')).not.toBeNull();
	});

	it('некомпактная полоса (правка, очередь) сохраняет нижний запас', () => {
		const bar = render({ compact: false }).querySelector('.cbar');
		expect(bar?.querySelectorAll(':scope > div')).toHaveLength(2);
	});

	it('название и действия на месте', () => {
		const el = render({ canPublish: true });
		expect(el.querySelector('.t')?.textContent).toBe('Кадр');
		expect(el.querySelector('.act')?.textContent).toBe('Отмена');
		expect(el.querySelector('.rt')?.textContent).toBe('Готово');
		expect(el.querySelector('.rt')?.hasAttribute('disabled')).toBe(false);
	});

	it('без готового кадра действие выключено', () => {
		expect(render().querySelector('.rt')?.hasAttribute('disabled')).toBe(true);
	});
});
