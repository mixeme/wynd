import type { CodeLine } from '$ui/admin/CodeBlock.svelte';

interface Focus {
	re: RegExp;
	cmt: string;
}

// Строки, которые чинят каждую поломку (C44: экран показывает, на какую
// строку смотреть; остальные приглушены — вставка всё равно целиком).
const FOCUS: Record<string, Focus> = {
	proxy_client: {
		re: /X-Forwarded-For|X-Real-IP|RequestHeader|forwardedHeaders/,
		cmt: 'настоящий адрес клиента'
	},
	proxy_body_limit: {
		re: /client_max_body_size|max_size|maxRequestBodyBytes|LimitRequestBody/,
		cmt: 'потолок вложений из настроек'
	},
	proxy_headers: { re: /X-Forwarded-Proto/, cmt: 'протокол для ссылок' },
	proxy_sse: {
		re: /proxy_buffering off|flush_interval|flushpackets/,
		cmt: 'иначе лента не обновляется'
	},
	proxy_timeout: { re: /read_timeout|timeout=\d+/, cmt: 'длинная загрузка не рвётся' }
};

/** Annotate proxy snippet lines for CodeBlock highlighting. */
export function annotateProxySnippet(snippet: string, fail?: string): CodeLine[] {
	const focus = fail ? FOCUS[fail] : undefined;
	return snippet.split('\n').map((text) => {
		if (focus) {
			return focus.re.test(text) ? { text, hi: true, cmt: focus.cmt } : { text, dim: true };
		}
		// Без поломки — все строки, которые что-то чинят, с пояснением.
		const hit = Object.values(FOCUS).find((f) => f.re.test(text));
		return hit ? { text, hi: true, cmt: hit.cmt } : { text };
	});
}
