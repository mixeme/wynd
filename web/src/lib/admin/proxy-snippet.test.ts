import { describe, expect, it } from 'vitest';
import { annotateProxySnippet } from './proxy-snippet';

const NGINX = [
	'location / {',
	'    proxy_set_header X-Real-IP $remote_addr;',
	'    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;',
	'    proxy_buffering off;',
	'    client_max_body_size 100m;',
	'}'
].join('\n');

describe('annotateProxySnippet', () => {
	it('marks only the lines of the failed check and dims the rest', () => {
		const lines = annotateProxySnippet(NGINX, 'proxy_client');
		expect(lines.filter((l) => l.hi).map((l) => l.text.trim())).toEqual([
			'proxy_set_header X-Real-IP $remote_addr;',
			'proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;'
		]);
		expect(lines.find((l) => l.text.includes('proxy_buffering'))?.dim).toBe(true);
	});

	it('marks every fixing line when no check failed', () => {
		const lines = annotateProxySnippet(NGINX);
		expect(lines.filter((l) => l.hi)).toHaveLength(4);
		expect(lines.some((l) => l.dim)).toBe(false);
	});
});
