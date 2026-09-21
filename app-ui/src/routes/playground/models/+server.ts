// The playground's model list, read from the data plane with the panel's own key.
//
// The list comes from `GET /api/v1/models` rather than from the management catalog, and the reason is
// what the screen promises: these are the strings this gateway key can actually route, resolved by the
// same resolver the send control will use (docs/SPEC-UI/001-SPEC-UI.md §6.15).

import type { RequestHandler } from './$types';
import { MODELS_PATH } from '$lib/server/playground';
import { playgroundContext } from '$lib/server/playground-context';
import { relay } from '$lib/server/playground-relay';

export const GET: RequestHandler = async ({ request, fetch }) => {
	const ready = await playgroundContext(request.headers.get('cookie'), fetch);
	if (!ready.ok) return ready.response;

	const { target, key } = ready.context;
	return relay(target, key, MODELS_PATH, null, fetch);
};
