// Which entities the live drawing puts a node on, including the combo band.
//
// Split from `usage-topology-view.ts` because the two answer different questions: this one decides which
// providers and combos deserve a node at all, that one decides where a node goes and what state it carries.
// Both are pure.
//
// The drawing's node set is not the API's. `configuredProviders` is the difference between a provider the
// gateway knows and one it could route anything through, and `configuredCombos` is the difference between a
// combo the operator has defined and one the box can still draw a readable label for.

import type { Combo } from './combo';
import type { Provider } from './provider';

/** A node's identity and the words to draw beside it. */
export type TopologyNodeEntry = {
	id: string;
	name: string;
};

/**
 * How many combo nodes the drawing places on its band.
 *
 * The band is half an ellipse, so it holds about half as many readable labels as the full ring the drawing
 * used before combos joined it: measured with `nodeShare`, eight nodes on each band still draw 98px boxes on
 * a desktop panel and twelve draw 62px. Past that the labels are initials rather than names. The panel says
 * which combos it left out instead of pretending the band holds them all.
 */
export const COMBO_MAX_NODES = 8;

function byId(a: TopologyNodeEntry, b: TopologyNodeEntry): number {
	if (a.id < b.id) return -1;
	return a.id > b.id ? 1 : 0;
}

/**
 * The providers the drawing puts a node on.
 *
 * The registry lists every provider the gateway knows, which is 94 entries in the reference, and a node per
 * entry would draw a map of things that have never routed. A provider is configured when it has at least
 * one stored endpoint, or when it needs none: a no-auth provider routes without a credential, so its
 * endpoint count stays at zero however much traffic it carries.
 *
 * Sorted by id rather than left in the API's order, so the drawing is the same drawing on every read and a
 * node does not move because the registry's ordering changed.
 */
export function configuredProviders(providers: Provider[]): TopologyNodeEntry[] {
	return providers
		.filter((provider) => provider.endpoint_count > 0 || provider.no_auth)
		.map((provider) => ({ id: provider.id, name: provider.name }))
		.sort(byId);
}

/**
 * The combos the drawing puts a node on: every one the operator defined, in name order, up to the cap.
 *
 * A combo needs no endpoint and no credential of its own (it is a list of models the gateway resolves)
 * so there is no "configured" test to apply beyond the one the read already made by returning it. Its
 * identity on the wire is its name, because that is the string a client addresses and the string the
 * in-flight marker carries, so `id` and `name` are the same value here rather than two columns that could
 * drift.
 */
export function configuredCombos(combos: Combo[]): TopologyNodeEntry[] {
	return combos
		.map((combo) => ({ id: combo.name, name: combo.name }))
		.sort(byId)
		.slice(0, COMBO_MAX_NODES);
}

/**
 * The registry's ids mapped to their display names, lowercased on the id.
 *
 * The breakdown table resolves a provider group key through this: the key is the id the API
 * grouped by, and `openai` is a name the operator has to translate. The full read is used rather than
 * `configuredProviders`, because a provider that has usage and no endpoint still has a name.
 */
export function providerNameMap(providers: { id: string; name: string }[]): Map<string, string> {
	return new Map(providers.map((provider) => [provider.id.toLowerCase(), provider.name]));
}
