// Forwarding a playground call, and translating what comes back.
//
// A success is piped, because the answer is the point: the status and the content type travel and the
// body is streamed rather than buffered, which is what makes an SSE answer arrive frame by frame.
//
// A failure is translated into the panel's envelope instead of being forwarded. The two planes share the
// code name `UNAUTHORIZED`, so a 401 passed through unchanged would read to the browser as an expired
// panel session and sign the operator out for a gateway key problem. The gateway's own error object
// travels inside the panel's, so a developer still sees the machine code the gateway wrote.

import { schemaDataPlaneError, type DataPlaneErrorDetail } from '$lib/schemas/playground';
import { dataPlaneUrl, playgroundError, playgroundHeaders } from './playground';

/**
 * Forwards a data-plane call and returns the gateway's answer as the browser will see it.
 *
 * `body` is null for a read (the models list) and a JSON string for the chat call. The credential comes
 * from the caller, which reads it once per request from the panel's environment.
 */
export async function relay(
	target: URL,
	key: string,
	path: string,
	body: string | null,
	fetchImpl: typeof fetch = fetch
): Promise<Response> {
	const headers = playgroundHeaders(key, body === null ? 'application/json' : 'text/event-stream');
	if (body !== null) headers.set('content-type', 'application/json');

	let upstream: Response;
	try {
		upstream = await fetchImpl(dataPlaneUrl(target, path), {
			method: body === null ? 'GET' : 'POST',
			headers,
			body: body ?? undefined,
			redirect: 'manual'
		});
	} catch {
		return playgroundError(
			'GATEWAY_UNREACHABLE',
			`The panel could not reach the gateway at ${target.origin}.`
		);
	}

	if (upstream.status === 401) {
		return failure(
			'GATEWAY_KEY_REFUSED',
			upstream,
			key,
			'The gateway refused the key the panel sent.'
		);
	}

	if (!upstream.ok) {
		return failure('GATEWAY_ERROR', upstream, key, `The gateway answered HTTP ${upstream.status}.`);
	}

	return pipe(upstream);
}

/** Builds the panel's failure response for a failed gateway answer, with the gateway's own words kept. */
async function failure(
	code: 'GATEWAY_KEY_REFUSED' | 'GATEWAY_ERROR',
	upstream: Response,
	key: string,
	fallback: string
): Promise<Response> {
	const detail = await readGatewayError(upstream);
	if (detail === null) return playgroundError(code, fallback);

	const message = redact(detail.message, key);
	return playgroundError(code, message.trim() === '' ? fallback : message, {
		gateway: { ...detail, message }
	});
}

/** Reads the gateway's error object out of a failed answer, or null when it wrote something else. */
async function readGatewayError(response: Response): Promise<DataPlaneErrorDetail | null> {
	const text = await response.text().catch(() => '');
	if (text.trim() === '') return null;

	try {
		const parsed = schemaDataPlaneError.safeParse(JSON.parse(text));
		return parsed.success ? parsed.data.error : null;
	} catch {
		return null;
	}
}

/**
 * Removes the panel's own key from a sentence before it reaches the browser.
 *
 * The gateway does not echo a credential today (its refusal reads "the gateway key is not valid"), but
 * this module forwards the gateway's words, so "the key never enters a response" is enforced here
 * rather than trusted to the far end's wording.
 */
function redact(message: string, key: string): string {
	return message.split(key).join('[redacted]');
}

/**
 * Streams an upstream answer to the browser.
 *
 * The content length is deliberately not copied: the runtime decompresses the upstream body, so a
 * forwarded length would describe bytes that no longer exist. `no-store` is set because a playground
 * answer is a paid call, and no cache should be able to serve it twice.
 */
function pipe(upstream: Response): Response {
	const headers = new Headers({ 'cache-control': 'no-store' });
	const contentType = upstream.headers.get('content-type');
	if (contentType !== null) headers.set('content-type', contentType);

	return new Response(upstream.body, { status: upstream.status, headers });
}
