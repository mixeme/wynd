import assert from 'node:assert/strict';
import test from 'node:test';
import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';

// esbuild берём из зависимостей web — как profiles.test.mjs берёт jsdom.
const require = createRequire(new URL('../../../../../web/package.json', import.meta.url));
const { build } = require('esbuild');
const libDir = fileURLToPath(new URL('./', import.meta.url));

const bundled = await build({
	stdin: { contents: 'export * from "./rollback";', resolveDir: libDir },
	bundle: true,
	format: 'esm',
	write: false
});
const { certNote, staleNote, removalNote } = await import(
	'data:text/javascript;base64,' + Buffer.from(bundled.outputFiles[0].text).toString('base64')
);

test('certNote speaks about revocation or its failure', () => {
	assert.equal(certNote(undefined), '');
	assert.equal(certNote('revoked'), ' Сертификат сайта отозван.');
	assert.match(certNote('failed'), /Отозвать сертификат сайта не получилось/);
	assert.match(certNote('failed'), /перестанет действовать сам/);
});

test('staleNote explains the three stale reasons only with a foreign proxy', () => {
	assert.equal(staleNote('', 'before'), '');
	assert.equal(staleNote('nginx', undefined), '');
	assert.match(staleNote('nginx', 'before'), /ещё не принял.*до удаления/);
	assert.match(staleNote('nginx', 'after'), /не принял.*перестала проходить проверку/);
	assert.match(staleNote('Caddy', 'reload'), /перечитать настройку Caddy не удалось/);
});

test('failed install rollback returns to the plan without foreign file words', () => {
	const text = removalNote({
		dropData: false,
		failed: true,
		proxyName: 'nginx',
		foreignSite: ['a.conf'],
		cert: undefined,
		stale: undefined
	});
	assert.equal(text, 'Установка откачена: Wynd с сервера убран, его данные оставлены. Docker и nginx остались.');
});

test('failed install rollback with dropped data and a stale proxy names the reason', () => {
	const text = removalNote({
		dropData: true,
		failed: true,
		proxyName: 'nginx',
		foreignSite: [],
		cert: 'revoked',
		stale: 'before'
	});
	assert.match(text, /^Установка откачена: Wynd и его данные с сервера убраны\. Docker и nginx остались\./);
	assert.match(text, /Сертификат сайта отозван\./);
	assert.match(text, /ещё не принял: в его настройке ошибка/);
});

test('removing a working wynd keeps data and certificate when they are ours', () => {
	const text = removalNote({
		dropData: false,
		failed: false,
		proxyName: '',
		foreignSite: [],
		cert: undefined,
		stale: undefined
	});
	assert.equal(text, 'Wynd с сервера убран, его данные и сертификат оставлены. Docker остался.');
});

test('removing with dropped data behind a foreign proxy lists left site files', () => {
	const text = removalNote({
		dropData: true,
		failed: false,
		proxyName: 'Apache',
		foreignSite: ['a.conf', 'b.conf'],
		cert: 'failed',
		stale: 'after'
	});
	assert.match(text, /^Wynd и его данные с сервера убраны\. Docker и Apache остались\./);
	assert.match(text, /Файл сайта a\.conf, b\.conf остался — его ставили не мы\./);
	assert.match(text, /Отозвать сертификат сайта не получилось/);
	assert.match(text, /перестала проходить проверку/);
});

test('removing without dropped data behind a foreign proxy keeps data but never mentions the certificate', () => {
	const text = removalNote({
		dropData: false,
		failed: false,
		proxyName: 'Caddy',
		foreignSite: ['site.conf'],
		cert: 'revoked',
		stale: undefined
	});
	assert.equal(
		text,
		'Wynd с сервера убран, его данные оставлены. Docker и Caddy остались. Файл сайта site.conf остался — его ставили не мы. Сертификат сайта отозван.'
	);
});
