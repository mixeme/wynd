/**
 * Модальность оверлея для клавиатуры и читалки (план 42, UI-1).
 *
 * `use:modal={{ ondismiss }}` на корне оверлея: при появлении запоминает
 * фокус и переносит его внутрь, Tab и Shift+Tab ходят по кругу внутри,
 * фокус снаружи возвращается внутрь, Escape зовёт `ondismiss`, при закрытии
 * фокус возвращается туда, где был. Роль и `aria-modal` ставит сам компонент.
 *
 * Оверлеи складываются в стек: клавиши и фокус слушает только верхний, иначе
 * Escape в листе над диалогом закрывал бы оба.
 *
 * Без `ondismiss` action ничего не делает: такой оверлей нарисован статично
 * (каталог `/dev/ui`), и ловушка захватила бы всю страницу.
 *
 * `initialFocus: 'last'` — фокус при открытии и при возврате снаружи идёт на
 * последний фокусируемый элемент: у кадра аватара это «Готово».
 */
import { setBackHandler } from '$lib/navigation/up';

export interface ModalOptions {
	ondismiss?: () => void;
	initialFocus?: 'first' | 'last';
}

const FOCUSABLE = [
	'a[href]',
	'button:not([disabled])',
	'input:not([disabled]):not([type="hidden"])',
	'select:not([disabled])',
	'textarea:not([disabled])',
	'[tabindex]:not([tabindex="-1"])',
	'[contenteditable="true"]'
].join(',');

const stack: HTMLElement[] = [];

export function focusableIn(node: HTMLElement): HTMLElement[] {
	return [...node.querySelectorAll<HTMLElement>(FOCUSABLE)].filter(
		(el) => !el.closest('[inert]') && el.getAttribute('aria-hidden') !== 'true'
	);
}

export function modal(node: HTMLElement, options: ModalOptions = {}) {
	if (!options.ondismiss) return {};
	let opts = options;
	const previous = document.activeElement instanceof HTMLElement ? document.activeElement : null;
	stack.push(node);
	if (!node.hasAttribute('tabindex')) node.setAttribute('tabindex', '-1');

	const isTop = () => stack[stack.length - 1] === node;

	function focusFirst() {
		const items = focusableIn(node);
		const target = opts.initialFocus === 'last' ? items.at(-1) : items[0];
		(target ?? node).focus();
	}

	function onKeydown(e: KeyboardEvent) {
		if (!isTop()) return;
		if (e.key === 'Escape') {
			if (!opts.ondismiss) return;
			e.preventDefault();
			e.stopPropagation();
			opts.ondismiss();
			return;
		}
		if (e.key !== 'Tab') return;
		const items = focusableIn(node);
		if (items.length === 0) {
			e.preventDefault();
			node.focus();
			return;
		}
		const idx = items.indexOf(document.activeElement as HTMLElement);
		let next: HTMLElement | undefined;
		if (idx === -1) next = e.shiftKey ? items[items.length - 1] : items[0];
		else if (e.shiftKey && idx === 0) next = items[items.length - 1];
		else if (!e.shiftKey && idx === items.length - 1) next = items[0];
		if (next) {
			e.preventDefault();
			next.focus();
		}
	}

	function onFocusIn(e: FocusEvent) {
		if (!isTop()) return;
		const target = e.target as Node | null;
		if (target && node.contains(target)) return;
		focusFirst();
	}

	document.addEventListener('keydown', onKeydown, true);
	document.addEventListener('focusin', onFocusIn);
	// Системная «Назад» закрывает оверлей, как Escape (план 46, C13).
	const dropBack = setBackHandler(() => opts.ondismiss?.());
	queueMicrotask(() => {
		if (isTop() && !node.contains(document.activeElement)) focusFirst();
	});

	return {
		update(next: ModalOptions = {}) {
			opts = next;
		},
		destroy() {
			document.removeEventListener('keydown', onKeydown, true);
			document.removeEventListener('focusin', onFocusIn);
			dropBack();
			const i = stack.indexOf(node);
			if (i >= 0) stack.splice(i, 1);
			if (previous && previous.isConnected) previous.focus();
		}
	};
}
