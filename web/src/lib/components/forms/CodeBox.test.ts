import { flushSync, mount, unmount } from 'svelte';
import { describe, expect, it } from 'vitest';
import CodeBox from './CodeBox.svelte';

function cells(value: string) {
	const target = document.createElement('div');
	const instance = mount(CodeBox, { target, props: { value } });
	flushSync();
	const out = [...target.querySelectorAll('.codebox u')].map((u) => ({
		char: u.textContent ?? '',
		active: u.classList.contains('act')
	}));
	unmount(instance);
	return out;
}

// Код набран целиком — курсора нет: в последней клетке черта курсора вставала
// второй строкой под цифрой. Неполный код — курсор в следующей пустой клетке.
describe('CodeBox', () => {
	it('shows no cursor when the code is complete', () => {
		const full = cells('123456');
		expect(full.map((c) => c.char).join('')).toBe('123456');
		expect(full.some((c) => c.active)).toBe(false);
	});

	it('puts the cursor in the next empty cell', () => {
		const part = cells('123');
		expect(part.findIndex((c) => c.active)).toBe(3);
	});

	it('keeps six digits from a pasted «123 456»', () => {
		expect(cells('123 456').map((c) => c.char).join('')).toBe('123456');
	});
});
