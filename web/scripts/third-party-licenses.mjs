#!/usr/bin/env node
// Собирает web/static/THIRD_PARTY_LICENSES.txt — уведомления сторонних
// лицензий (LIC-3). MIT/BSD/ISC/Apache требуют сохранять текст уведомления в
// поставке, а бандлер его вырезает; файл отдаётся клиентом по ссылке
// «лицензии компонентов» рядом с «исходный код».
//
// Источники — штатные инструменты, без сетевых установок:
//   npm ls --omit=dev --all --json   — боевые зависимости клиента;
//   go list -deps ./cmd/wynd         — модули, попавшие в бинарь.
// Тексты берутся из node_modules и кеша модулей Go. Зависимость без файла
// лицензии — отказ: её нужно разобрать руками, а не выпустить молча.
//
// Запуск: npm run licenses (в web/) или make licenses.

import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const webRoot = path.resolve(fileURLToPath(import.meta.url), '..', '..');
const repoRoot = path.resolve(webRoot, '..');
// Путь переопределяется только тестом, который сверяет свежесть файла.
const outFile = process.env.WYND_LICENSES_OUT
	? path.resolve(process.env.WYND_LICENSES_OUT)
	: path.join(webRoot, 'static', 'THIRD_PARTY_LICENSES.txt');

// LICENSE, LICENCE, UNLICENSE, COPYING, NOTICE, а также MIT-LICENCE.txt и
// LICENSE-GO — префикс отделяется дефисом, точкой или подчёркиванием.
const LICENSE_FILE = /(^|[-_.])(un)?licen[sc]e|^copying|^notice/i;

function licenseText(dir) {
	let names;
	try {
		names = fs.readdirSync(dir);
	} catch {
		return null;
	}
	const files = names
		.filter((n) => LICENSE_FILE.test(n))
		.filter((n) => {
			try {
				return fs.statSync(path.join(dir, n)).isFile();
			} catch {
				return false;
			}
		})
		.sort();
	if (files.length === 0) return null;
	return files
		.map((n) => `--- ${n} ---\n${fs.readFileSync(path.join(dir, n), 'utf8').trimEnd()}`)
		.join('\n\n');
}

function collectNpm() {
	// package-lock.json, а не `npm ls`: Node 24 не запускает npm.cmd без
	// оболочки, а в замке и так записано, что боевое, а что dev.
	const lock = JSON.parse(fs.readFileSync(path.join(webRoot, 'package-lock.json'), 'utf8'));
	const found = new Map();
	for (const [where, node] of Object.entries(lock.packages ?? {})) {
		if (where === '' || node.dev || node.devOptional) continue;
		const name = node.name ?? where.slice(where.lastIndexOf('node_modules/') + 'node_modules/'.length);
		if (!found.has(name)) found.set(name, { version: node.version, where });
	}

	const out = [];
	for (const [name, { version, where }] of [...found].sort(([a], [b]) => a.localeCompare(b))) {
		const dir = path.join(webRoot, ...where.split('/'));
		const pkgPath = path.join(dir, 'package.json');
		let declared = '';
		let homepage = '';
		if (fs.existsSync(pkgPath)) {
			const pkg = JSON.parse(fs.readFileSync(pkgPath, 'utf8'));
			declared = typeof pkg.license === 'string' ? pkg.license : (pkg.license?.type ?? '');
			homepage = pkg.homepage ?? pkg.repository?.url ?? pkg.repository ?? '';
		}
		out.push({ name, version, declared, homepage, text: licenseText(dir) });
	}
	return out;
}

function collectGo() {
	const format = '{{if .Module}}{{.Module.Path}}|{{.Module.Version}}|{{.Module.Dir}}{{end}}';
	const found = new Map();
	// Список пакетов зависит от GOOS (x/sys/windows против x/sys/unix): берём
	// объединение платформ, на которых собирается бинарь.
	for (const goos of ['linux', 'windows']) {
		const raw = execFileSync('go', ['list', '-deps', '-f', format, './cmd/wynd'], {
			cwd: repoRoot,
			encoding: 'utf8',
			maxBuffer: 64 * 1024 * 1024,
			// Без shell: на Windows оболочка съедает фигурные скобки в -f.
			env: { ...process.env, GOOS: goos }
		});
		for (const line of raw.split(/\r?\n/)) {
			if (!line.trim()) continue;
			const [modPath, version, dir] = line.split('|');
			// Свой модуль — под AGPL, он не «стороннее».
			if (!version || !dir) continue;
			if (!found.has(modPath)) found.set(modPath, { version, dir });
		}
	}
	return [...found]
		.sort(([a], [b]) => a.localeCompare(b))
		.map(([name, { version, dir }]) => ({
			name,
			version,
			declared: '',
			homepage: `https://${name}`,
			text: licenseText(dir)
		}));
}

function render(groups) {
	const lines = [
		'Сторонние компоненты Wynd',
		'',
		'Wynd распространяется под GNU AGPL v3 (файл LICENSE в исходниках).',
		'Ниже — уведомления компонентов, входящих в поставку: клиентские пакеты',
		'npm и модули Go, вкомпилированные в сервер. Файл собран автоматически',
		'(web/scripts/third-party-licenses.mjs), правки вносятся в скрипт.',
		'',
		'Исходный код компонентов под MPL-2.0 берётся по адресу проекта,',
		'указанному рядом с компонентом.',
		''
	];
	for (const [title, items] of groups) {
		lines.push('='.repeat(72), title, '='.repeat(72), '');
		for (const item of items) {
			const head = `${item.name}@${item.version}${item.declared ? ` — ${item.declared}` : ''}`;
			lines.push('-'.repeat(72), head);
			if (item.homepage) lines.push(String(item.homepage));
			lines.push(
				'',
				item.text ??
					`Пакет не содержит файла лицензии; в package.json объявлено: ${item.declared}.`,
				''
			);
		}
	}
	return lines.join('\n').replace(/\n{3,}/g, '\n\n') + '\n';
}

const npm = collectNpm();
const go = collectGo();
// Пакет без файла лицензии, но с объявленной в package.json, попадает в файл
// строкой об объявленной лицензии. Пакет, о лицензии которого не известно
// ничего, — отказ: такое разбирают руками, а не выпускают молча.
const unknown = [...npm, ...go]
	.filter((i) => !i.text && !i.declared)
	.map((i) => `${i.name}@${i.version}`);
if (unknown.length > 0) {
	console.error('licenses: лицензия неизвестна:\n  ' + unknown.join('\n  '));
	process.exit(1);
}

fs.mkdirSync(path.dirname(outFile), { recursive: true });
fs.writeFileSync(
	outFile,
	render([
		['Клиент (npm, боевые зависимости)', npm],
		['Сервер (модули Go)', go]
	]),
	'utf8'
);
console.log(`licenses: ${npm.length} npm + ${go.length} Go → ${path.relative(repoRoot, outFile)}`);
