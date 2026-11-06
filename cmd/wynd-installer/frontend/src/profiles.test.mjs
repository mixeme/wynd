import assert from 'node:assert/strict';
import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';
import { readFile } from 'node:fs/promises';
import path from 'node:path';
import test from 'node:test';

const require = createRequire(new URL('../../../../web/package.json', import.meta.url));
const { JSDOM } = require('jsdom');
const { build } = require('esbuild');
const { compile } = createRequire(new URL('../package.json', import.meta.url))('svelte/compiler');
const root = fileURLToPath(new URL('../../../../', import.meta.url));
const frontend = path.join(root, 'cmd/wynd-installer/frontend');
const applicationCSS = await readFile(path.join(frontend, 'src/app.css'), 'utf8');
const result = await build({
	stdin: {
		contents:
			"import {mount} from 'svelte';import Window from './src/Window.svelte';import {SCENES} from './src/gallery/scenes';import {makeBackend} from './src/gallery/fake';window.workshopScenes=SCENES;window.makeWorkshopBackend=makeBackend;window.mountWorkshop=()=>mount(Window,{target:document.getElementById('app')});",
		resolveDir: frontend
	},
	bundle: true,
	format: 'iife',
	write: false,
	conditions: ['browser'],
	alias: {
		$ui: path.join(root, 'web/src/lib/components'),
		$lib: path.join(root, 'web/src/lib')
	},
	plugins: [
		{
			name: 'svelte-dom',
			setup(b) {
				b.onResolve({ filter: /^svelte(?:\/|$)/ }, (args) =>
					args.pluginData?.dedup
						? undefined
						: b.resolve(args.path, {
								resolveDir: frontend,
								kind: args.kind,
								pluginData: { dedup: true }
							})
				);
				b.onLoad({ filter: /\.svelte$/ }, async ({ path: filename }) => ({
					contents: compile(await readFile(filename, 'utf8'), {
						filename,
						generate: 'client',
						css: 'injected'
					}).js.code,
					resolveDir: path.dirname(filename)
				}));
			}
		}
	]
});
const code = result.outputFiles[0].text;
const profile = (id, extra = {}) => ({
	id,
	name: `Сервер ${id}`,
	host: `${id}.example.org`,
	user: 'root',
	use_keys: true,
	remember: false,
	auto_connect: false,
	...extra
});
const report = {
	ok: true,
	report: {
		findings: [{ id: 'wynd', level: 'ok', text: 'Wynd' }],
		addresses: [],
		raw: ''
	}
};
const page = {
	ok: true,
	state: {
		installed: true,
		container: true,
		running: true,
		domain: 'site.example.org',
		version: '1.0.0',
		free_bytes: 1024 ** 3
	},
	host: 'a.example.org',
	copy_bytes: 1024,
	latest: '1.0.1',
	live_copy: true,
	can_build: true,
	built: false
};

async function open(t, rows, overrides = {}) {
	const calls = [];
	let stored = structuredClone(rows);
	const base = {
		Profiles: () => structuredClone(stored),
		Version: () => '0.3.1',
		CheckSelfUpdate: () => ({ available: false }),
		HasSavedPassword: () => false,
		SetAutoConnect: (id, on) => {
			stored.find((p) => p.id === id).auto_connect = on;
		},
		SaveProfile: (p) => {
			const saved = { ...p, id: p.id || 'added' };
			stored = stored.filter((p) => p.id !== saved.id).concat(saved);
			return saved;
		},
		RemoveProfile: (id) => {
			stored = stored.filter((p) => p.id !== id);
		},
		Connect: (id) => ({ status: 'connected', host: `${id}.example.org` }),
		Inspect: () => report,
		ServerPage: () => page,
		CopyText: () => true,
		Disconnect: () => {},
		FinishMove: () => 'b',
		MoveSelect: () => ({ status: 'connected' }),
		MovePlan: () => ({
			ok: true,
			domain: 'site.example.org',
			version: '1.0.0',
			old_ip: '192.0.2.1',
			new_ip: '192.0.2.2',
			size: 1024,
			data_only: false
		})
	};
	const raw = new Proxy(
		{ ...base, ...overrides },
		{
			get(target, key) {
				if (!(key in target)) return async () => undefined;
				return async (...args) => {
					calls.push([key, ...args]);
					return target[key](...args);
				};
			}
		}
	);
	const dom = new JSDOM('<div id="app"></div>', {
		url: 'https://workshop.test',
		runScripts: 'dangerously',
		pretendToBeVisual: true
	});
	const stylesheet = dom.window.document.createElement('style');
	stylesheet.textContent = applicationCSS;
	dom.window.document.head.appendChild(stylesheet);
	dom.window.go = { main: { App: raw } };
	dom.window.structuredClone = structuredClone;
	dom.window.eval(code);
	dom.window.mountWorkshop();
	t.after(() => dom.window.close());
	const settle = async () => {
		for (let i = 0; i < 6; i++) await new Promise((r) => setTimeout(r, 0));
	};
	const visible = (e) => !e.closest('.ins-hidden,[hidden],[inert]');
	const elements = (sel) => [...dom.window.document.querySelectorAll(sel)].filter(visible);
	const click = async (text) => {
		const button = elements('button,label').find(
			(e) => e.textContent.replace(/\s+/g, ' ').trim() === text
		);
		assert.ok(button, `no visible «${text}»: ${elements('h2').map((e) => e.textContent)}`);
		button.click();
		await settle();
	};
	const set = async (el, value) => {
		el.value = value;
		el.dispatchEvent(
			new dom.window.Event(el.tagName === 'SELECT' ? 'change' : 'input', {
				bubbles: true
			})
		);
		await settle();
	};
	const select = async (id) => {
		dom.window.document.querySelector(`.ins-server-entry[data-profile="${id}"]`).click();
		await settle();
	};
	await settle();
	return {
		dom,
		calls,
		click,
		set,
		select,
		elements,
		settle,
		stored: () => stored
	};
}

test('first launch and form preserve the password area while excluding it from focus', async (t) => {
	const ui = await open(t, []);
	assert.equal(ui.elements('h2')[0].textContent, 'Ваши серверы');
	await ui.click('Добавить сервер');
	const fields = ui.dom.window.document.querySelector('.ins-password-fields');
	assert.equal(fields.inert, true);
	assert.equal(fields.getAttribute('aria-hidden'), 'true');
	await ui.set(ui.elements('select')[0], 'password');
	assert.equal(fields.getAttribute('aria-hidden'), 'false');
	const auto = ui
		.elements('label')
		.find((e) => e.textContent.includes('Подключаться при запуске'))
		.querySelector('input');
	assert.equal(auto.disabled, true);
	await ui.click('Запомнить пароль на этом компьютере');
	assert.equal(auto.disabled, false);
});

test('unknown fingerprint of one auto-connection does not block another or disabling its flag', async (t) => {
	const ui = await open(
		t,
		[profile('a', { auto_connect: true }), profile('b', { auto_connect: true })],
		{
			Connect: (id) =>
				id === 'a'
					? {
							status: 'unknown_host',
							fingerprint: 'SHA256:untrusted',
							host: 'a.example.org'
						}
					: { status: 'connected', host: 'b.example.org' }
		}
	);
	assert.equal(ui.calls.filter((c) => c[0] === 'Connect').length, 2);
	assert.equal(
		ui.calls.some((c) => c[0] === 'ConfirmHost'),
		false
	);
	await ui.click('Подключаться при запуске');
	assert.deepEqual(
		ui.calls.find((c) => c[0] === 'SetAutoConnect'),
		['SetAutoConnect', 'a', false]
	);
	assert.equal(ui.stored()[1].auto_connect, true);
	assert.ok(ui.elements('h2').some((e) => e.textContent.includes('Это ваш сервер')));
	await ui.select('b');
	assert.equal(ui.elements('h2')[0].textContent, 'Wynd работает');
});

test('background update remains attached to its profile across server and archive navigation', async (t) => {
	const progress = {
		running: true,
		done: false,
		steps: [{ id: 'image', title: 'Получаем образ', status: 'running' }],
		log: []
	};
	const ui = await open(
		t,
		[profile('a', { auto_connect: true }), profile('b', { auto_connect: true })],
		{ StartUpdate: () => progress, InstallProgress: () => progress }
	);
	await ui.click('Обновить');
	await ui.click('Обновить');
	assert.equal(ui.calls.find((c) => c[0] === 'StartUpdate')[1], 'a');
	await ui.select('b');
	assert.equal(ui.elements('h2')[0].textContent, 'Wynd работает');
	await ui.click('Архивы');
	await ui.click('Серверы');
	await ui.select('a');
	assert.equal(ui.elements('h2')[0].textContent, 'Обновляем');
	assert.equal(ui.calls.filter((c) => c[0] === 'StartUpdate').length, 1);
	const forget = ui.elements('button').find((e) => e.textContent.trim() === 'Убрать из списка');
	assert.equal(forget, undefined, 'busy operation keeps only the approved operation footer');
	assert.equal(ui.elements('.ins-connection-badge')[0].textContent, 'Подключён');
	assert.ok(ui.elements('.ins-server-entry')[0].textContent.includes('Обновление'));
	// C71: полосы с выдуманным процентом нет — только этапы и последняя строка.
	assert.equal(ui.elements('.meter').length, 0);
	await ui.click('Подключаться при запуске');
	assert.equal(ui.stored()[0].auto_connect, false);
});

test('new profile connects using its own stable ID and ephemeral password', async (t) => {
	const ui = await open(t, [], {
		Connect: () => ({
			status: 'password_required',
			message: 'Для входа нужен пароль'
		})
	});
	await ui.click('Добавить сервер');
	await ui.set(ui.elements('input')[0], 'Семейный');
	await ui.set(ui.elements('input')[1], 'new.example.org');
	await ui.set(ui.elements('select')[0], 'password');
	await ui.set(ui.elements('input[type=password]')[0], 'ephemeral-secret');
	await ui.click('Подключиться');
	const save = ui.calls.find((c) => c[0] === 'SaveProfile');
	assert.equal(JSON.stringify(save).includes('ephemeral-secret'), false);
	assert.deepEqual(
		ui.calls.find((c) => c[0] === 'Connect'),
		['Connect', 'added', 'ephemeral-secret', false]
	);
	assert.equal(ui.elements('h2')[0].textContent, 'Для входа нужен пароль');
	assert.equal(
		ui.calls.some((c) => c[0] === 'StartInstall'),
		false
	);
});

test('saved move destinations identify both SSH roles and adding returns to the source', async (t) => {
	const ui = await open(t, [profile('a', { auto_connect: true }), profile('b')]);
	await ui.click('Перенести');
	assert.equal(ui.elements('h2')[0].textContent, 'На какой сервер переезжаем');
	assert.equal(ui.elements('label').filter((e) => e.textContent.includes('Сервер a')).length, 0);
	await ui.click('Добавить сервер');
	await ui.set(ui.elements('input')[0], 'Третий сервер');
	await ui.set(ui.elements('input')[1], 'third.example.org');
	await ui.click('Подключиться');
	assert.equal(ui.elements('h2')[0].textContent, 'На какой сервер переезжаем');
	const dest = ui.elements('label').find((e) => e.textContent.includes('Сервер b'));
	dest.click();
	await ui.settle();
	assert.ok(ui.elements('.ins-move-pair')[0].textContent.includes('a.example.org'));
	assert.ok(ui.elements('.ins-move-pair')[0].textContent.includes('b.example.org'));
	await ui.click('Подключиться и проверить');
	assert.deepEqual(
		ui.calls.find((c) => c[0] === 'MoveSelect'),
		['MoveSelect', 'a', 'b', '']
	);
	assert.ok(ui.elements('h2')[0].textContent.includes('Что сделаем'));
});

test('removing a local profile calls no remote removal command', async (t) => {
	const ui = await open(t, [profile('a'), profile('b')]);
	await ui.click('Убрать из списка');
	await ui.click('Убрать из списка');
	assert.deepEqual(
		ui.calls.find((c) => c[0] === 'RemoveProfile'),
		['RemoveProfile', 'a']
	);
	assert.equal(ui.stored().length, 1);
	assert.equal(
		ui.calls.some((c) => ['Connect', 'Rollback', 'Service'].includes(c[0])),
		false
	);
});

test('all gallery scenarios use the current compiled window without remote servers', async (t) => {
	const source = await open(t, []);
	const scenes = source.dom.window.workshopScenes;
	const fake = source.dom.window.makeWorkshopBackend;
	const errors = [];
	for (const scene of scenes) {
		const ui = await open(t, [], fake(scene.fake ?? {}));
		try {
			for (const act of scene.acts ?? []) {
				if ('destination' in act) {
					const input = ui.elements('input[type=radio]').find((e) => e.value === act.destination);
					assert.ok(input, 'missing destination');
					input.click();
					await ui.settle();
				} else if ('tab' in act) await ui.click('Архивы');
				else if ('click' in act) {
					if (act.nth) {
						const matches = ui
							.elements('button,label')
							.filter((e) => e.textContent.replace(/\s+/g, ' ').trim() === act.click);
						assert.ok(matches[act.nth], `missing ${act.click} #${act.nth}`);
						matches[act.nth].click();
						await ui.settle();
					} else await ui.click(act.click);
				} else {
					const input = ui.elements('input').find((e) => e.placeholder === act.fill);
					assert.ok(input, `missing ${act.fill}`);
					await ui.set(input, act.value);
				}
			}
		} catch (e) {
			errors.push(`${scene.name}: ${e.message}`);
		} finally {
			ui.dom.window.close();
		}
	}
	assert.deepEqual(errors, []);
});

test('changed fingerprints stay local until explicit confirmation', async (t) => {
	const ui = await open(
		t,
		[profile('a', { auto_connect: true }), profile('b', { auto_connect: true })],
		{
			Connect: (id) =>
				id === 'a'
					? {
							status: 'changed_host',
							fingerprint: 'SHA256:new',
							previous_fingerprint: 'SHA256:old'
						}
					: { status: 'connected' },
			ConfirmChangedHost: () => ({ status: 'connected' })
		}
	);
	assert.equal(ui.elements('h2')[0].textContent, 'У сервера другой отпечаток');
	assert.ok(ui.elements('.ins-mono').some((e) => e.textContent.includes('SHA256:old')));
	assert.equal(
		ui.calls.some((c) => c[0] === 'ConfirmChangedHost'),
		false
	);
	await ui.click('Подключаться при запуске');
	assert.equal(ui.stored()[0].auto_connect, false);
	await ui.click('Проверил, доверять новому');
	assert.deepEqual(
		ui.calls.find((c) => c[0] === 'ConfirmChangedHost'),
		['ConfirmChangedHost', 'a']
	);
});

test('new key profile with auto-connect starts exactly one connection', async (t) => {
	const ui = await open(t, []);
	await ui.click('Добавить сервер');
	await ui.set(ui.elements('input')[0], 'Новый');
	await ui.set(ui.elements('input')[1], 'new.example.org');
	await ui.click('Подключаться при запуске');
	await ui.click('Подключиться');
	assert.equal(ui.calls.filter((c) => c[0] === 'Connect').length, 1);
	assert.equal(ui.calls.find((c) => c[0] === 'Connect')[1], 'added');
});

test('editing access uses the new values and never reconnects a different profile', async (t) => {
	const ui = await open(t, [profile('a', { use_keys: false, remember: false }), profile('b')], {
		HasSavedPassword: () => true
	});
	await ui.click('Изменить доступ');
	await ui.set(ui.elements('input')[1], 'changed.example.org');
	await ui.click('Запомнить пароль на этом компьютере');
	await ui.click('Сохранить и подключиться');
	const updated = ui.stored().find((p) => p.id === 'a');
	assert.equal(updated.host, 'changed.example.org');
	assert.equal(updated.remember, true);
	assert.equal(ui.calls.filter((c) => c[0] === 'SaveProfile').length, 1);
	assert.deepEqual(
		ui.calls.find((c) => c[0] === 'Connect'),
		['Connect', 'a', '', false]
	);
});

test('actual application CSS hides inactive sections and preserves the desktop frame', async (t) => {
	const ui = await open(t, [profile('a'), profile('b')]);
	const doc = ui.dom.window.document;
	const style = (element) => ui.dom.window.getComputedStyle(element);
	assert.equal(
		style(doc.querySelector('.ins-content')).display,
		'flex',
		'desktop content must be a flex column'
	);
	assert.equal(
		style(doc.querySelector('.ins-workspace')).display,
		'flex',
		'workspace must participate in the frame'
	);
	assert.equal(
		style(doc.querySelector('.ins-archive-view')).display,
		'none',
		'archives must really be hidden while Servers is selected'
	);
	assert.equal(
		style(doc.querySelectorAll('.ins-profile-view')[1]).display,
		'none',
		'unselected server must really be hidden'
	);
	await ui.click('Архивы');
	assert.equal(
		style(doc.querySelector('.ins-content > .ins')).display,
		'none',
		'server section must really be hidden while Archives is selected'
	);
	assert.equal(style(doc.querySelector('.ins-archive-view')).display, 'flex');
	await ui.click('Серверы');
	assert.equal(style(doc.querySelector('.ins-archive-view')).display, 'none');
});

test('approved first launch keeps its primary action in the bottom panel', async (t) => {
	const ui = await open(t, []);
	assert.ok(
		ui.elements('.ins-server-rail')[0].textContent.includes('Сохранённых серверов пока нет.')
	);
	const footer = ui.elements('.ins-pane > .ins-foot')[0];
	assert.ok(footer, 'first launch must have its approved bottom action panel');
	assert.ok(footer.textContent.includes('Добавить сервер'));
	assert.equal(ui.elements('.ins-welcome button').length, 0);
	assert.equal(ui.dom.window.getComputedStyle(footer).marginTop, 'auto');
});
