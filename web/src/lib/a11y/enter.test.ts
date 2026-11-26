import { describe, expect, it, vi } from 'vitest';
import { enterSubmit } from './enter';

function key(init: KeyboardEventInit): KeyboardEvent {
	return new KeyboardEvent('keydown', { cancelable: true, ...init });
}

// Инвариант: Enter в поле отправляет действие, остальные клавиши и ввод IME —
// нет. Без этого Enter на экране входа молча ничего не делал (26.11).
describe('enterSubmit', () => {
	it('зовёт действие по Enter и гасит браузерную отправку', () => {
		const fn = vi.fn();
		const event = key({ key: 'Enter' });
		enterSubmit(fn)(event);
		expect(fn).toHaveBeenCalledOnce();
		expect(event.defaultPrevented).toBe(true);
	});

	it('не трогает другие клавиши', () => {
		const fn = vi.fn();
		for (const name of ['Tab', 'a', 'NumpadEnter', 'Escape']) {
			enterSubmit(fn)(key({ key: name }));
		}
		expect(fn).not.toHaveBeenCalled();
	});

	it('не отправляет во время ввода IME', () => {
		const fn = vi.fn();
		enterSubmit(fn)(key({ key: 'Enter', isComposing: true }));
		const legacy = key({ key: 'Enter' });
		Object.defineProperty(legacy, 'keyCode', { value: 229 });
		enterSubmit(fn)(legacy);
		expect(fn).not.toHaveBeenCalled();
	});
});
