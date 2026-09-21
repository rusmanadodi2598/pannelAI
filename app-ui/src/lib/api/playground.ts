// Playground calls (docs/SPEC-UI/001-SPEC-UI.md §6.15).
//
// Both calls go to the panel's own routes rather than to `/api/v1`, and the reason is the whole design:
// the credential that serves them lives on the panel's server, so the browser has nothing to send and
// nothing to leak. The models read is an ordinary JSON call. The send is a stream, so it reads frames
// instead of a body, and the frame reading lives in `playground-reader.ts`.
//
// This module is the surface the screen imports: the two calls, their addresses, and the handler shape.
// The types are re-exported rather than declared here, so a caller has one import and the reader keeps
// ownership of what it calls back through.

import {
	schemaPlaygroundFailure,
	schemaPlaygroundModels,
	type PlaygroundFailure,
	type PlaygroundModels,
	type PlaygroundRequest
} from '$lib/schemas/playground';
import { isAbort, localFailure, readStream } from './playground-reader';
import type { PlaygroundStreamHandlers } from './playground-reader';
import { reportUnauthorized } from './client';

export type { PlaygroundEndReason, PlaygroundStreamHandlers } from './playground-reader';

/** The panel's own prefix for this screen. Nothing here touches `/api/v1` from the browser. */
export const PLAYGROUND_BASE = '/playground';

/** The two addresses this module calls. They are routes of the panel, not of the gateway. */
export const PLAYGROUND_MODELS_PATH = `${PLAYGROUND_BASE}/models`;
export const PLAYGROUND_CHAT_PATH = `${PLAYGROUND_BASE}/chat`;

/**
 * The result of a playground read.
 *
 * Deliberately not the panel's `ApiResult`: these routes answer with the playground's own envelope, whose
 * code set is outside the management vocabulary the shared client knows, so the failure travels as that
 * envelope rather than as an `ApiError` carrying a code its union does not have. The session-expiry
 * signal is still reported through the shared client, because that behaviour belongs in one place.
 */
export type PlaygroundResult<T> = { ok: true; data: T } | { ok: false; failure: PlaygroundFailure };

/**
 * The models the gateway says this key can route.
 *
 * This call does its own fetch rather than going through `apiRequest`, and the reason is the envelope:
 * the playground's failures use the panel's own code vocabulary, which the shared client does not know,
 * so a `PLAYGROUND_KEY_MISSING` would arrive as an unreadable body.
 */
export async function fetchPlaygroundModels(): Promise<PlaygroundResult<PlaygroundModels>> {
	let response: Response;
	try {
		response = await fetch(PLAYGROUND_MODELS_PATH, {
			headers: { accept: 'application/json' },
			credentials: 'same-origin'
		});
	} catch {
		return {
			ok: false,
			failure: localFailure('GATEWAY_UNREACHABLE', 'The panel could not be reached.')
		};
	}

	reportUnauthorized(response.status);

	if (!response.ok) {
		return { ok: false, failure: await readFailure(response) };
	}

	const payload = await response.json().catch(() => null);
	const parsed = schemaPlaygroundModels.safeParse(payload);

	if (!parsed.success) {
		return {
			ok: false,
			failure: localFailure(
				'INTERNAL_ERROR',
				'The panel answered a shape this screen does not read.'
			)
		};
	}

	return { ok: true, data: parsed.data };
}

/**
 * Sends one message and reads the streamed answer.
 *
 * The failure cases are separated by where they happened: a body the panel refused or a panel that could
 * not be dialled ends the call without a stream, while everything after the content-type check is the
 * reader's business. `onStart` reports the status of the answer that is about to be read, which is the
 * upstream status §6.15 rule 4 asks the screen to show.
 */
export async function streamPlaygroundChat(
	request: PlaygroundRequest,
	handlers: PlaygroundStreamHandlers,
	signal?: AbortSignal
): Promise<void> {
	let response: Response;
	try {
		response = await fetch(PLAYGROUND_CHAT_PATH, {
			method: 'POST',
			headers: { 'content-type': 'application/json', accept: 'text/event-stream' },
			body: JSON.stringify(request),
			credentials: 'same-origin',
			signal
		});
	} catch (cause) {
		if (isAbort(cause)) {
			handlers.onEnd('stopped');
			return;
		}
		handlers.onFailure(localFailure('GATEWAY_UNREACHABLE', 'The panel could not be reached.'));
		return;
	}

	if (!response.ok) {
		handlers.onFailure(await readFailure(response));
		return;
	}

	if (!(response.headers.get('content-type') ?? '').includes('text/event-stream')) {
		handlers.onFailure(
			localFailure('INTERNAL_ERROR', 'The panel answered a shape this screen does not read.')
		);
		return;
	}

	handlers.onStart?.(response.status);
	await readStream(response, handlers, signal);
}

/** The panel's failure envelope out of a failed response, or a sentence when it wrote something else. */
async function readFailure(response: Response): Promise<PlaygroundFailure> {
	const text = await response.text().catch(() => '');

	try {
		const parsed = schemaPlaygroundFailure.safeParse(JSON.parse(text));
		if (parsed.success) return parsed.data;
	} catch {
		// Fall through to the sentence below: a body that is not JSON is not a failure the panel wrote.
	}

	return localFailure(
		'GATEWAY_ERROR',
		`The panel answered HTTP ${response.status} without a readable error.`
	);
}
