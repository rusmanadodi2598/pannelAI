// The playground's send control, server side: validate the body, then forward the call with the key.
//
// Validation happens here rather than in the browser alone because this route is reachable directly,
// and a route that spends a paid credential must refuse a body it cannot read before it spends
// anything (docs/SPEC-UI/001-SPEC-UI.md §6.15).

import type { RequestHandler } from './$types';
import { CHAT_PATH, chatRequestBody, playgroundError } from '$lib/server/playground';
import { playgroundContext } from '$lib/server/playground-context';
import { relay } from '$lib/server/playground-relay';
import { schemaPlaygroundRequest } from '$lib/schemas/playground';

export const POST: RequestHandler = async ({ request, fetch }) => {
	let raw: unknown;
	try {
		raw = await request.json();
	} catch {
		return playgroundError('VALIDATION_ERROR', 'The request body is not readable JSON.');
	}

	const parsed = schemaPlaygroundRequest.safeParse(raw);
	if (!parsed.success) {
		const issue = parsed.error.issues[0];
		return playgroundError(
			'VALIDATION_ERROR',
			`${issue.path.join('.') || 'body'}: ${issue.message}`
		);
	}

	const ready = await playgroundContext(request.headers.get('cookie'), fetch);
	if (!ready.ok) return ready.response;

	const { target, key } = ready.context;
	return relay(target, key, CHAT_PATH, chatRequestBody(parsed.data), fetch);
};
