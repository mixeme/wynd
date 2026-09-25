import type { CodeLine } from '$ui/admin/CodeBlock.svelte';

const HIGHLIGHT_PATTERNS = [
	/client_max_body_size/,
	/max_size/,
	/maxRequestBodyBytes/,
	/proxy_buffering off/,
	/flush_interval/,
	/X-Forwarded-Proto/,
	/proxy_set_header X-Forwarded-For/,
	/read_timeout/
];

/** Annotate proxy snippet lines for CodeBlock highlighting. */
export function annotateProxySnippet(snippet: string, fail?: string): CodeLine[] {
	return snippet.split('\n').map((text) => {
		const hi = HIGHLIGHT_PATTERNS.some((re) => re.test(text));
		let cmt: string | undefined;
		if (fail === 'proxy_body_limit' && /client_max_body_size|max_size|maxRequestBodyBytes/.test(text)) {
			cmt = ' ← лимит тела';
		} else if (fail === 'proxy_sse' && /proxy_buffering off|flush_interval/.test(text)) {
			cmt = ' ← буферизация SSE';
		} else if (fail === 'proxy_headers' && /X-Forwarded-Proto/.test(text)) {
			cmt = ' ← протокол для ссылок';
		}
		return { text, hi: hi || Boolean(cmt), cmt };
	});
}
