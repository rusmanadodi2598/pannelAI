// Fixtures shared by the playground's server tests.
//
// The pieces are here rather than in each test file because three of them need the same stub: a fetch
// that records what it was asked for (the credential placement is one of the things under test), and the
// four response shapes the gateway and the panel speak.

export const TARGET = new URL('http://gw.test:9090');
export const KEY = 'sk-abcdefghijklmnopqrstuvwxyz234567';

export type Call = {
	url: string;
	method: string;
	authorization: string | null;
	cookie: string | null;
	body: string | null;
};

export function recordingFetch(answer: () => Response | Promise<Response>): {
	calls: Call[];
	impl: typeof fetch;
} {
	const calls: Call[] = [];
	const impl = (async (input: RequestInfo | URL, init?: RequestInit) => {
		const headers = new Headers(init?.headers);
		calls.push({
			url: String(input),
			method: init?.method ?? 'GET',
			authorization: headers.get('authorization'),
			cookie: headers.get('cookie'),
			body: typeof init?.body === 'string' ? init.body : null
		});
		return answer();
	}) as typeof fetch;

	return { calls, impl };
}

export function jsonResponse(status: number, body: unknown): Response {
	return new Response(JSON.stringify(body), {
		status,
		headers: { 'content-type': 'application/json' }
	});
}

export function sseResponse(frames: string[]): Response {
	return new Response(frames.join(''), {
		status: 200,
		headers: { 'content-type': 'text/event-stream' }
	});
}

/** The status body app-serv really sends (§7.2): all three members, not just the one the panel reads. */
export function statusResponse(authenticated: boolean): Response {
	return jsonResponse(200, {
		authenticated,
		require_login: !authenticated,
		password_configured: true
	});
}

/** A data-plane failure body, the shape §4 fixes for the gateway's own errors. */
export function dataPlaneFailure(
	status: number,
	code: string,
	message: string,
	type = 'invalid_request_error'
): Response {
	return jsonResponse(status, { error: { code, type, message } });
}
