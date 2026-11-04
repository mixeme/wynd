import { test } from 'node:test';
import assert from 'node:assert/strict';
import { ReleaseRequests } from './release-requests.ts';

const tracker = () => {
	let n = 0;
	return new ReleaseRequests(() => `request-${++n}`);
};

test('late response and progress from the same tag cannot replace a newer check', () => {
	const requests = tracker();
	const old = requests.beginCheck();
	const current = requests.beginCheck();
	assert.equal(requests.isCurrentCheck(old), false);
	assert.equal(requests.isCurrentCheck(current), true);
	const p = {
		phase: 'check',
		tag: 'same-tag',
		request_id: old,
		check_id: 'old-check'
	};
	assert.equal(requests.acceptsProgress(p, 'same-tag'), false);
	assert.equal(requests.acceptsProgress({ ...p, request_id: current }, 'same-tag'), true);
	assert.equal(requests.acceptsProgress({ ...p, request_id: current }, 'different-tag'), false);
});

test('signing retries have independent event identities even with the same Check', () => {
	const requests = tracker();
	const checked = requests.beginCheck();
	const old = requests.beginSign();
	const current = requests.beginSign();
	const p = {
		phase: 'sign',
		tag: 'same-tag',
		request_id: old,
		check_id: 'verified-check'
	};
	assert.equal(requests.isCurrentSign(checked, old), false);
	assert.equal(requests.acceptsProgress(p, 'same-tag', 'verified-check'), false);
	assert.equal(
		requests.acceptsProgress({ ...p, request_id: current }, 'same-tag', 'verified-check'),
		true
	);
	assert.equal(
		requests.acceptsProgress({ ...p, request_id: current }, 'same-tag', 'other-check'),
		false
	);
	requests.beginCheck();
	assert.equal(requests.isCurrentSign(checked, current), false);
});

test('leaving the release invalidates every outstanding result and event', () => {
	const requests = tracker();
	const checked = requests.beginCheck();
	const signed = requests.beginSign();
	requests.clear();
	assert.equal(requests.isCurrentCheck(checked), false);
	assert.equal(requests.isCurrentCheck(''), false);
	assert.equal(requests.isCurrentSign(checked, signed), false);
	assert.equal(
		requests.acceptsProgress({ phase: 'check', tag: 'a', request_id: checked }, 'a'),
		false
	);
});
