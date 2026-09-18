// Routing derivation for the endpoint detail drawer (docs/SPEC-UI/001-SPEC-UI.md §6.2).
//
// The drawer answers "why did my request fail over" by naming the key the router would spend. §6.2 is
// explicit that this is derived from the fields the API returns rather than re-simulated in the panel, so
// this orders by priority and skips what the API has already marked unusable, and nothing more. Modelling
// health decay, rate-limit windows, or failover ordering here would be a second implementation of the
// router, and the two would disagree the first time either one changed.

import { ENDPOINT_STATUS_ACTIVE, type EndpointKey } from '$lib/schemas/endpoint';

/**
 * The key the router would spend, or null when none can be spent.
 *
 * `available` is the API's own verdict on a key's health and `status` is its stored state, so a key that
 * is disabled or unhealthy is skipped here for the same reason the router skips it. A tie on priority
 * keeps the earlier key in the API's order, which is the order the router sees.
 */
export function routingKey(keys: readonly EndpointKey[]): EndpointKey | null {
	let best: EndpointKey | null = null;

	for (const key of keys) {
		if (!key.available || key.status !== ENDPOINT_STATUS_ACTIVE) continue;
		if (best === null || key.priority < best.priority) best = key;
	}

	return best;
}

// The credential kinds whose removal the API refuses when it would leave nothing to route with. The API
// normalizes the registry's `apikey` spelling to `api_key` before storing, but the panel reads what the
// API returns, so both are accepted here rather than trusting one spelling.
const API_KEY_AUTH_TYPES = new Set(['api_key', 'apikey']);

/**
 * Whether deleting this key is the case SPEC-API §7.5 refuses with CONFLICT: the endpoint authenticates
 * with a key, and this is its last active one.
 *
 * SPEC-UI §6.2 asks the panel to disable the delete control with an inline note rather than offer an
 * action that cannot succeed. This is a hint for the render only; the panel still handles a CONFLICT
 * response, because the state can change between the render and the click.
 */
export function isLastActiveApiKey(
	keys: readonly EndpointKey[],
	keyId: string,
	authType: string
): boolean {
	if (!API_KEY_AUTH_TYPES.has(authType)) return false;

	const active = keys.filter((key) => key.status === ENDPOINT_STATUS_ACTIVE);

	return active.length === 1 && active[0].id === keyId;
}
