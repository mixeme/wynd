import type { CodeLine } from '$ui/admin/CodeBlock.svelte';

const HIGHLIGHT_PATTERNS = [
	/client_max_body_size/,
	/max_size/,
	/maxRequestBodyBytes/,
	/proxy_buffering off/,
	/flush_interval/,
	/X-Forwarded-Proto/,
	/X-Forwarded-For/,
	/proxy_set_header X-Real-IP/,
	/header_up X-Real-IP/,
	/forwardedHeaders/,
	/read_timeout/
];

/** Annotate proxy snippet lines for CodeBlock highlighting. */
export function annotateProxySnippet(snippet: string, fail?: string): CodeLine[] {
	return snippet.split('\n').map((text) => {
		const hi = HIGHLIGHT_PATTERNS.some((re) => re.test(text));
		let cmt: string | undefined;
		if (/client_max_body_size|max_size|maxRequestBodyBytes/.test(text)) {
			cmt = 'потолок вложений из настроек';
		} else if (/proxy_buffering off|flush_interval/.test(text)) {
			cmt = 'иначе лента не обновляется';
		} else if (/read_timeout/.test(text)) {
			cmt = 'длинная загрузка не рвётся';
		}
		if (fail === 'proxy_body_limit' && /client_max_body_size|max_size|maxRequestBodyBytes/.test(text)) {
			cmt = 'потолок вложений из настроек';
		} else if (fail === 'proxy_sse' && /proxy_buffering off|flush_interval/.test(text)) {
			cmt = 'иначе лента не обновляется';
		} else if (fail === 'proxy_headers' && /X-Forwarded-Proto/.test(text)) {
			cmt = 'протокол для ссылок';
		} else if (
			fail === 'proxy_client' &&
			/proxy_set_header X-Forwarded-For|proxy_set_header X-Real-IP|header_up X-Forwarded-For|header_up X-Real-IP|forwardedHeaders/.test(
				text
			)
		) {
			cmt = undefined;
		}
		return { text, hi: hi || Boolean(cmt), cmt };
	});
}
