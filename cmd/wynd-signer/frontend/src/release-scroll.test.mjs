import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readReleaseScroll, releaseScrollTarget } from './release-scroll.ts';

// Окно 525 px, экран вырос с 723 до 1186 px — как при этапах подписи.
const VIEW = 525;

test('позицию берут перед каждым обновлением, а не один раз при нажатии', () => {
	const where = readReleaseScroll(100, 723, VIEW);
	// Нажали «Подписать» у начала (0), но человек уже прокрутил до этапов.
	assert.equal(releaseScrollTarget(where, 1186, VIEW, false), 100);
});

test('у конца окно остаётся у конца при росте списка', () => {
	const where = readReleaseScroll(198, 723, VIEW);
	assert.equal(where.atEnd, true);
	assert.equal(releaseScrollTarget(where, 1186, VIEW, false), 1186 - VIEW);
});

test('экран, который помещался целиком, после роста остаётся у начала', () => {
	const where = readReleaseScroll(0, VIEW, VIEW);
	assert.equal(where.atEnd, false);
	assert.equal(releaseScrollTarget(where, 1186, VIEW, false), 0);
});

test('ниже нового конца позиция подрезается, но не прыгает к началу', () => {
	const where = readReleaseScroll(600, 1186, VIEW);
	assert.equal(releaseScrollTarget(where, 900, VIEW, false), 900 - VIEW);
});

test('итог подписи показывается у конца экрана', () => {
	const where = readReleaseScroll(321, 1186, VIEW);
	assert.equal(releaseScrollTarget(where, 800, VIEW, true), 800);
});
