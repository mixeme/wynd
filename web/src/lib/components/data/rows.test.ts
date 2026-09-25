import { createRawSnippet, flushSync, mount, unmount, type Component } from 'svelte';
import { describe, expect, it } from 'vitest';
import CircleRow from './CircleRow.svelte';
import MemberRow from './MemberRow.svelte';
import SearchResultRow from './SearchResultRow.svelte';
import ServerRow from './ServerRow.svelte';
import SettingsRow from './SettingsRow.svelte';

const snippet = (html: string) =>
	createRawSnippet(() => ({ render: () => html, setup: () => () => {} }));
const noop = () => {};

// Разметка строки без служебных комментариев Svelte и без пробелов между
// тегами (в flex-строке и между блоками они не видны): сравниваем то, что
// видит браузер.
function render<P extends Record<string, unknown>>(component: Component<P>, props: P): string {
	const target = document.createElement('div');
	const instance = mount(component, { target, props });
	flushSync();
	const html = target.innerHTML.replace(/<!--[^>]*-->/g, '').replace(/>\s+</g, '><');
	unmount(instance);
	return html;
}

// Инвариант (план 42, UI-5): четыре строки собраны на общей основе, и их
// разметка во всех вариантах та же, что до слияния, — слепки сняты со старых
// компонентов. Меняется разметка — меняется и слепок, осознанно.
describe('row components markup', () => {
	it('ServerRow', () => {
		for (const variant of ['select', 'ok', 'warn', 'info'] as const) {
			for (const onclick of [undefined, noop]) {
				for (const card of [false, true]) {
					expect(
						render(ServerRow, { name: 'Дом', subtitle: 'wynd.example', variant, card, onclick, style: 'color: red' })
					).toMatchSnapshot(`${variant} click=${Boolean(onclick)} card=${card}`);
				}
			}
		}
	});

	it('SettingsRow', () => {
		const cases: Array<[string, Record<string, unknown>]> = [
			['plain', { title: 'Имя' }],
			['subtitle', { title: 'Имя', subtitle: 'Аня' }],
			['icon value', { title: 'Имя', icon: 'user', value: 'Аня' }],
			['link', { title: 'Выйти', link: true }],
			['no chevron', { title: 'Имя', chevron: false }],
			['click', { title: 'Имя', subtitle: 'Аня', value: 'x', onclick: noop }],
			['click link', { title: 'Выйти', link: true, onclick: noop }],
			['control', { title: 'Звук', control: snippet('<span class="sw"></span>') }],
			['control subtitle', { title: 'Звук', subtitle: 'в круге', icon: 'bell', control: snippet('<span class="sw"></span>') }],
			['control link', { title: 'Звук', link: true, control: snippet('<span class="sw"></span>') }]
		];
		for (const [name, props] of cases) {
			expect(
				render(SettingsRow as unknown as Component<Record<string, unknown>>, { class: 'c', style: 'color: red', ...props })
			).toMatchSnapshot(name);
		}
	});

	it('MemberRow', () => {
		const cases: Array<[string, Record<string, unknown>]> = [
			['plain', { initial: 'А', name: 'Аня' }],
			['subtitle faded', { initial: 'А', name: 'Аня', subtitle: 'владелец', color: '#c47', faded: true }],
			['menu', { initial: 'А', name: 'Аня', menu: true, onmenu: noop }],
			['click', { initial: 'А', name: 'Аня', subtitle: 'x', onclick: noop, menu: true, onmenu: noop }],
			['click faded', { initial: 'А', name: 'Аня', onclick: noop, faded: true }]
		];
		for (const [name, props] of cases) {
			expect(
				render(MemberRow as unknown as Component<Record<string, unknown>>, { style: 'color: red', ...props })
			).toMatchSnapshot(name);
		}
	});

	it('CircleRow', () => {
		const base = { initial: 'С', name: 'Семья', preview: 'Аня: привет', time: '12:30', style: 'color: red' };
		const cases: Array<[string, Record<string, unknown>]> = [
			['plain', {}],
			['badge color', { badge: 3, color: '#c47' }],
			['click', { onclick: noop, badge: 0 }],
			['card', { card: true, class: 'c' }],
			['card click actions', { card: true, onclick: noop, actionLabel: 'В группу', onaction: noop, actionLabel2: 'Открепить', onaction2: noop }],
			['card action without handler', { card: true, actionLabel: 'В группу' }]
		];
		for (const [name, props] of cases) {
			expect(
				render(CircleRow as unknown as Component<Record<string, unknown>>, { ...base, ...props })
			).toMatchSnapshot(name);
		}
	});

	it('SearchResultRow', () => {
		const preview = snippet('<span>текст <b>найдено</b></span>');
		const cases: Array<[string, Record<string, unknown>]> = [
			['plain', {}],
			['thumb placeholder', { thumb: true }],
			['thumb variant', { thumb: true, thumbVariant: 'p3' }],
			['thumb url', { thumb: true, thumbUrl: 'blob:x' }],
			['click thumb url', { thumb: true, thumbUrl: 'blob:x', onclick: noop }],
			['click', { onclick: noop }]
		];
		for (const [name, props] of cases) {
			expect(
				render(SearchResultRow as unknown as Component<Record<string, unknown>>, {
					author: 'Аня',
					time: 'вчера',
					preview,
					style: 'color: red',
					...props
				})
			).toMatchSnapshot(name);
		}
	});
});
