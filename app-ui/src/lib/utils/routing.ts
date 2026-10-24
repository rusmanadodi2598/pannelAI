// Routing derivation for the endpoint detail drawer (docs/SPEC-UI/001-SPEC-UI.md §6.2).
//
// The drawer answers "why did my request fail over" by naming the key the router would spend. §6.2 is
// explicit that this is derived from the fields the API returns rather than re-simulated in the panel, so
// this orders by priority and skips what the API has already marked unusable, and nothing more. Modelling
// health decay, rate-limit windows, or failover ordering here would be a second implementation of the
// router, and the two would disagree the first time either one changed.

import { failureClassLabel } from '$lib/schemas/settings-failure-classes';
import {
	ENDPOINT_STATUS_ACTIVE,
	RESET_SOURCE_PHRASES,
	type EndpointKey
} from '$lib/schemas/endpoint';

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
// API returns, so both are accepted here rather than trusting one spelling. `service_account` joins them
// because its credential is a row in the same keys table (`domain.UsesStoredKey`): deleting the last
// active one would leave the endpoint unable to mint its bearer.
const API_KEY_AUTH_TYPES = new Set(['api_key', 'apikey', 'service_account']);

/**
 * Whether deleting this key is the case SPEC-API §7.5 refuses with CONFLICT: the endpoint authenticates
 * with a stored key, and this is its last active one.
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

/**
 * The one line a parked credential reads under its countdown: what it failed as, and whose wait it is.
 *
 * §6.2 asks the drawer to answer why a key is out of rotation, and the countdown alone answers how long.
 * A reset the vendor stated in its own header or body is the vendor's; `default` is a wait the gateway
 * chose from the class ladder. Telling those apart is the point of the field: before it, one five minute
 * park covered both a short throttle and an allowance spent until next Tuesday, so a spent key came back
 * into rotation days early and burned a failover attempt every cycle. The signature id rides at the end
 * because it is what an operator quotes when a vendor's pattern is not recognised yet.
 *
 * A key that names no class reads nothing here. An absent field is a key that never failed, or one parked
 * before the classifier existed, and a cause the API never sent is not the panel's to invent.
 */
export function parkProvenance(key: EndpointKey): string {
	if (key.last_error_class === undefined) return '';

	const parts = [`parked as ${failureClassLabel(key.last_error_class)}`];
	if (key.reset_source !== undefined) {
		// A source the panel has never seen reads as the word the API sent, the rule `status` and
		// `auth_type` already follow: a value this build cannot name is a gap in the build, not a blank.
		parts.push(RESET_SOURCE_PHRASES[key.reset_source] ?? key.reset_source);
	}

	const line = parts.join(', ');
	return key.signature_id === undefined ? line : `${line} (${key.signature_id})`;
}
