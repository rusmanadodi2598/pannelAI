// Server hook: forward the API prefix, serve everything else from SvelteKit.
//
// A misconfigured environment returns a readable JSON error instead of an empty 500, because the
// panel is the surface an operator sees first when the target is wrong.

import type { Handle } from '@sveltejs/kit';
import { EnvError } from '$lib/schemas/env';
import { panelEnv } from '$lib/server/config';
import { forwardToUpstream, shouldProxy } from '$lib/server/proxy';

export const handle: Handle = async ({ event, resolve }) => {
	if (!shouldProxy(event.url.pathname)) return resolve(event);

	try {
		return await forwardToUpstream(event.request, new URL(panelEnv().PANEL_API_TARGET));
	} catch (cause) {
		const message =
			cause instanceof EnvError
				? `Panel configuration error. ${cause.message}`
				: 'The panel could not reach the gateway.';

		return new Response(JSON.stringify({ error: { code: 'INTERNAL_ERROR', message } }), {
			status: 500,
			headers: { 'content-type': 'application/json' }
		});
	}
};
