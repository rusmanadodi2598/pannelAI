// The gateway base address, as this panel server knows it (docs/SPEC-UI/001-SPEC-UI.md §5.2).
//
// The browser cannot derive this. `location.origin` is the panel's own address rather than the gateway's,
// and it is a bind-all address like `http://0.0.0.0:3000` whenever the operator opens the panel that way,
// which is not an address any client can call. The panel server is the one component that holds the
// gateway's address (PANEL_API_TARGET, the target it forwards `/api/v1` to), so it answers with it.
//
// Session-gated, because the value is an internal address and a signed-in operator is the only caller that
// should learn it. The check is the panel's shared one, asked of the gateway that owns the session.
//
// No framework import: the module answers with a `Response` the way the playground's own envelope does,
// which keeps this layer testable without a request context.

import { panelEnv } from './config';
import { API_PREFIX } from './proxy';
import { sessionIsValid } from './session';

function failure(status: number, code: string, message: string): Response {
	return new Response(JSON.stringify({ error: { code, message } }), {
		status,
		headers: { 'content-type': 'application/json', 'cache-control': 'no-store' }
	});
}

/**
 * The gateway base address for a caller with a live session, or the response that says why not.
 *
 * The order is configuration first, so a misconfigured panel names its own problem instead of reporting a
 * missing session.
 */
export async function apiBaseResponse(
	cookie: string | null,
	fetchImpl: typeof fetch = fetch
): Promise<Response> {
	let target: URL;
	try {
		target = new URL(panelEnv().PANEL_API_TARGET);
	} catch (cause) {
		const message = cause instanceof Error ? cause.message : 'The panel configuration is invalid.';
		return failure(500, 'INTERNAL_ERROR', message);
	}

	if (!(await sessionIsValid(target, cookie, fetchImpl))) {
		return failure(401, 'UNAUTHORIZED', 'Your session is not valid. Sign in again.');
	}

	// `origin` rather than the configured value itself: the forwarder rebuilds the path from `/api/v1` on,
	// so a path in the target is never called and reporting it would describe an address the panel does not
	// actually use.
	return new Response(JSON.stringify({ base_url: `${target.origin}${API_PREFIX}` }), {
		status: 200,
		headers: { 'content-type': 'application/json', 'cache-control': 'no-store' }
	});
}
