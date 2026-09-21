// The playground's server side: the only place in the panel that holds a gateway key.
//
// SPEC-UI §6.15 decides the model and this module is that decision in code: the browser never holds a
// credential, so the panel's own server performs the data-plane call and attaches the `Authorization`
// header from PANEL_PLAYGROUND_KEY. Three rules follow, and each is enforced rather than trusted to the
// route files:
//
//   The credential is attached in exactly one function (`playgroundHeaders`), so a second call site
//   cannot invent a second placement.
//
//   The key never enters a response: the gateway's answer is piped through (`playground-relay.ts`), and
//   every failure carries a code and a sentence.
//
//   The route is not a new auth path. Every call first asks the gateway whether the caller's session is
//   valid (`playground-context.ts`), so an unauthenticated browser cannot spend the key, and the
//   playground key grants nothing beyond the data plane the gateway already authenticates (§6.15 rule 3).

import type { PlaygroundErrorCode, PlaygroundRequest } from '$lib/schemas/playground';

/** The data-plane paths this screen calls. Both are §7.15 routes; no playground-specific route exists. */
export const MODELS_PATH = '/api/v1/models';
export const CHAT_PATH = '/api/v1/chat/completions';

/** The panel's own status for each code. A gateway failure is the panel's upstream failing, so it is a 502. */
const ERROR_STATUS: Record<PlaygroundErrorCode, number> = {
	PLAYGROUND_KEY_MISSING: 503,
	UNAUTHORIZED: 401,
	VALIDATION_ERROR: 400,
	GATEWAY_UNREACHABLE: 502,
	GATEWAY_KEY_REFUSED: 502,
	GATEWAY_ERROR: 502,
	INTERNAL_ERROR: 500
};

type FailureExtra = { gateway?: { code: string; type: string; message: string } };

/** A panel failure, in the panel's envelope. The only response shape the playground produces on error. */
export function playgroundError(
	code: PlaygroundErrorCode,
	message: string,
	extra: FailureExtra = {}
): Response {
	const body = { error: { code, message, ...extra } };
	return new Response(JSON.stringify(body), {
		status: ERROR_STATUS[code],
		headers: { 'content-type': 'application/json', 'cache-control': 'no-store' }
	});
}

/** The absolute data-plane URL for a path. Built the same way the /api/v1 forwarder builds its own. */
export function dataPlaneUrl(target: URL, path: string): URL {
	return new URL(path, target);
}

/**
 * The outbound headers, and the only line in the panel that writes the credential.
 *
 * The accept type is the caller's because a streamed call asks for `text/event-stream`; the credential
 * placement is not.
 */
export function playgroundHeaders(key: string, accept: string): Headers {
	return new Headers({ accept, authorization: `Bearer ${key}` });
}

/**
 * The OpenAI chat body the panel sends.
 *
 * One user message, streamed, with `include_usage` set: the usage chunk is the last frame of the stream
 * and the only place the token counts appear, so a screen that wants to report them has to ask for it.
 */
export function chatRequestBody(request: PlaygroundRequest): string {
	return JSON.stringify({
		model: request.model,
		messages: [{ role: 'user', content: request.message }],
		stream: true,
		stream_options: { include_usage: true }
	});
}
