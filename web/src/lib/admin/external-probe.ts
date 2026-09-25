import { apiPath } from '$lib/api/client';

export interface ExternalReport {
	https_ok: boolean;
	https_ms: number;
	redirect_permanent: boolean;
	from_outside: boolean;
	proxy_https: boolean;
	proxy_body_limit_ok: boolean;
	proxy_sse_ok: boolean;
}

interface ProbeInfo {
	proto: string;
	peer_loopback: boolean;
	body_probe_bytes?: number;
}

const MIN_BODY_PROBE_BYTES = 2 * 1024 * 1024;
const SSE_TIMEOUT_MS = 5000;

function publicFetch(origin: string, path: string, init?: RequestInit): Promise<Response> {
	return fetch(apiPath(origin, path), { cache: 'no-store', ...init });
}

async function probeHTTPS(origin: string): Promise<{ ok: boolean; ms: number }> {
	const start = performance.now();
	try {
		const res = await publicFetch(origin, '/instance');
		return { ok: res.ok, ms: Math.round(performance.now() - start) };
	} catch {
		return { ok: false, ms: Math.round(performance.now() - start) };
	}
}

async function probeHeaders(origin: string): Promise<ProbeInfo | undefined> {
	try {
		const res = await publicFetch(origin, '/probe');
		if (!res.ok) return undefined;
		return (await res.json()) as ProbeInfo;
	} catch {
		return undefined;
	}
}

async function probeBodyLimit(origin: string, maxBytes: number): Promise<boolean> {
	const size = maxBytes > 0 ? maxBytes : MIN_BODY_PROBE_BYTES;
	const body = new Uint8Array(size);
	try {
		const res = await publicFetch(origin, '/probe/body', {
			method: 'PUT',
			body,
			headers: { 'Content-Type': 'application/octet-stream' }
		});
		return res.ok;
	} catch {
		return false;
	}
}

async function probeSSE(origin: string): Promise<boolean> {
	const controller = new AbortController();
	const timer = setTimeout(() => controller.abort(), SSE_TIMEOUT_MS);
	try {
		const res = await publicFetch(origin, '/probe/sse', {
			headers: { Accept: 'text/event-stream' },
			signal: controller.signal
		});
		if (!res.ok || !res.body) return false;
		const reader = res.body.getReader();
		const { value } = await reader.read();
		reader.cancel().catch(() => {});
		return value !== undefined && value.length > 0;
	} catch {
		return false;
	} finally {
		clearTimeout(timer);
	}
}

function bodyProbeSize(probe: ProbeInfo | undefined, attachmentMaxBytes: number): number {
	const fromProbe = probe?.body_probe_bytes;
	if (fromProbe && fromProbe > 0) return fromProbe;
	const cap = attachmentMaxBytes > 0 ? attachmentMaxBytes : MIN_BODY_PROBE_BYTES;
	return Math.min(cap, MIN_BODY_PROBE_BYTES);
}

/** Browser-side checks for HTTPS and reverse-proxy reachability. */
export async function collectExternalReport(
	origin: string,
	attachmentMaxBytes = MIN_BODY_PROBE_BYTES
): Promise<ExternalReport> {
	const https = await probeHTTPS(origin);
	const probe = await probeHeaders(origin);
	const bodyBytes = bodyProbeSize(probe, attachmentMaxBytes);
	const [bodyOK, sseOK] = await Promise.all([
		probeBodyLimit(origin, bodyBytes),
		probeSSE(origin)
	]);

	return {
		https_ok: https.ok,
		https_ms: https.ms,
		redirect_permanent: false,
		from_outside: probe ? !probe.peer_loopback : false,
		proxy_https: probe?.proto === 'https',
		proxy_body_limit_ok: bodyOK,
		proxy_sse_ok: sseOK
	};
}
