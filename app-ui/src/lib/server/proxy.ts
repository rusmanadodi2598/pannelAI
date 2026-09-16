// Same-origin forwarding of /api/v1 from the panel server to app-serv.
//
// Why the panel forwards instead of the browser calling app-serv directly: the session cookie is
// HttpOnly and SameSite=Lax, which only works when the browser sees one origin
// (docs/SPEC-UI/001-SPEC-UI.md §3.1). One code path serves dev and production.
//
// Hop-by-hop headers are dropped per RFC 9110 §7.6.1: forwarding `host`, `content-length`, or
// `transfer-encoding` would describe the wrong connection.

const HOP_BY_HOP = new Set([
	'connection',
	'keep-alive',
	'proxy-authenticate',
	'proxy-authorization',
	'te',
	'trailer',
	'transfer-encoding',
	'upgrade',
	'host',
	'content-length'
]);

export const API_PREFIX = '/api/v1';

export function shouldProxy(pathname: string): boolean {
	return pathname === API_PREFIX || pathname.startsWith(`${API_PREFIX}/`);
}

export function buildForwardUrl(target: URL, requestUrl: URL): URL {
	return new URL(`${requestUrl.pathname}${requestUrl.search}`, target);
}

export function buildForwardHeaders(headers: Headers, target: URL): Headers {
	const forwarded = new Headers();

	for (const [key, value] of headers) {
		if (!HOP_BY_HOP.has(key.toLowerCase())) forwarded.set(key, value);
	}

	forwarded.set('host', target.host);
	forwarded.set('x-forwarded-host', new URL(headers.get('referer') ?? target.origin).host);

	return forwarded;
}

export async function forwardToUpstream(request: Request, target: URL): Promise<Response> {
	const init: RequestInit & { duplex?: 'half' } = {
		method: request.method,
		headers: buildForwardHeaders(request.headers, target),
		redirect: 'manual'
	};

	if (request.method !== 'GET' && request.method !== 'HEAD') {
		init.body = request.body;
		init.duplex = 'half';
	}

	return fetch(buildForwardUrl(target, new URL(request.url)), init);
}
