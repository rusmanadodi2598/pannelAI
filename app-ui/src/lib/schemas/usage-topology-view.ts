// The live drawing's derivations (docs/DRAFT/012-USAGE-LIVE-UI-READINESS.md F3).
//
// Split from `usage-live-view.ts` because the two answer different questions: that file is about the
// connection and what the panel may say about it, this one is about where a node goes and which providers
// get one. Both are pure, so both are testable without a DOM, a clock, or a socket. The beam that travels
// along an active edge's line is a third question, and it lives in `usage-beam.ts`.
//
// Two functions, and each exists because the wire's shape and the screen's question differ:
//
//   `configuredProviders` turns the registry into the node set. The registry is every provider the gateway
//   knows, which is not the same list as the providers that could route anything.
//
//   `topologyNodes` turns that set plus the live states into positions, so the drawing holds no arithmetic.
//   It also answers how wide a node may be drawn, because that is arithmetic over the positions and the
//   drawing must not hold any: on a narrow box the nodes shrink with it instead of colliding (draft 018).

import type { Provider } from './provider';

export type TopologyState = 'active' | 'last' | 'error' | 'idle';

export type TopologyNode = {
	/** The provider id, which is what the node and its edge are keyed by. */
	id: string;
	name: string;
	/** Position as a percentage of the drawing box, so the drawing needs no measurement to place it. */
	x: number;
	y: number;
	state: TopologyState;
};

export type TopologyLayout = {
	nodes: TopologyNode[];
	/** The drawing box's height in pixels, from the node count. */
	height: number;
	/** The width one node may take, as a share of the drawing box's width (draft 018). */
	nodeShare: number;
};

export type TopologyLive = {
	/** The provider ids with a request in flight. */
	active: string[];
	/** The provider of the most recent completed request, or an empty string. */
	last: string;
	/** The provider the gateway last reported an error for, or an empty string. */
	error: string;
};

// The ellipse, as a percentage of the drawing box. Percentages rather than pixels so the same layout fills
// a phone and a desktop panel without measuring anything, and the edge drawing uses the same numbers, so a
// line always ends where its node is.
const RADIUS_X = 40;
const RADIUS_Y = 38;

// The box grows with the node count so labels do not collide, bounded at both ends: below the floor the
// drawing is a strip, and above the ceiling it is taller than the screen it sits on.
const MIN_HEIGHT = 320;
const MAX_HEIGHT = 640;
const HEIGHT_PER_NODE = 22;
const HEIGHT_BASE = 120;

// The node box at its largest, in the pixels the drawing's own metrics are written in: the widest node the
// panel draws today (padding 8 + dot 8 + gap 8 + label 96 + border 2), and the node height the reference
// fork lays its own nodes out at (`ProviderTopology.js:265` on `origin/master`, `nodeH = 30`).
//
// `NODE_MAX_WIDTH` is exported because the drawing's unit is derived from it: `--u` is the box's width over
// the width at which a node reaches this cap, so the drawing shrinks below the cap and never grows past it.
export const NODE_MAX_WIDTH = 130;
const NODE_HEIGHT = 30;

// The widest a node may be as a share of the drawing box's width, and the clearance kept between two nodes
// that share a row. The share is what `UsageTopologyDrawing.svelte` sizes every metric from, so a narrow box
// draws a smaller drawing rather than a collided one, which is what the reference's fitView does with its
// whole canvas.
const NODE_MAX_SHARE = 0.2;
const NODE_CLEARANCE = 0.15;

function lower(value: string): string {
	return value.toLowerCase();
}

/**
 * The share of the drawing box's width one node may take, so that two nodes on one row never touch.
 *
 * Only pairs that share a row can collide whatever their horizontal distance, so the row is what this looks
 * at: a pair whose vertical distance is a whole node height apart is clear at any width. The result is a
 * share of the box's width, which is unitless on purpose: it holds at every width the box can take, and the
 * drawing caps it at `NODE_MAX_WIDTH` pixels once the box is wide enough.
 */
function nodeShare(positions: { x: number; y: number }[], height: number): number {
	const row = (NODE_HEIGHT / height) * 100;
	let tightest = Number.POSITIVE_INFINITY;
	for (let i = 0; i < positions.length; i += 1) {
		for (let j = i + 1; j < positions.length; j += 1) {
			if (Math.abs(positions[i].y - positions[j].y) >= row) continue;
			tightest = Math.min(tightest, Math.abs(positions[i].x - positions[j].x));
		}
	}
	if (tightest === Number.POSITIVE_INFINITY) return NODE_MAX_SHARE;
	return Math.min(NODE_MAX_SHARE, (tightest / 100) * (1 - NODE_CLEARANCE));
}

/**
 * The drawing: one node per provider, on an ellipse around the gateway.
 *
 * States are exclusive and take the reference fork's precedence: a provider with a request in flight is
 * active whatever else is true of it, then an error outranks the last-request mark, because "the gateway
 * reported an error here" is the more useful fact to have on screen.
 *
 * Provider ids are compared without case, because the frame's `error_provider` and the registry's ids are
 * both strings from a gateway the panel does not control, and a difference of case is not a difference of
 * provider.
 */
export function topologyNodes(
	providers: { id: string; name: string }[],
	live: TopologyLive
): TopologyLayout {
	const count = providers.length;
	const height = Math.min(MAX_HEIGHT, Math.max(MIN_HEIGHT, HEIGHT_BASE + HEIGHT_PER_NODE * count));

	if (count === 0) return { nodes: [], height, nodeShare: NODE_MAX_SHARE };

	const active = new Set(live.active.map(lower));
	const last = lower(live.last);
	const error = lower(live.error);

	const nodes = providers.map((provider, index) => {
		// Evenly from the top, clockwise, which is the reference's own start so a reader who knows that
		// drawing finds the same first node here.
		const angle = -Math.PI / 2 + (2 * Math.PI * index) / count;
		const id = lower(provider.id);

		let state: TopologyState = 'idle';
		if (active.has(id)) state = 'active';
		else if (id === error) state = 'error';
		else if (id === last) state = 'last';

		return {
			id: provider.id,
			name: provider.name,
			x: 50 + RADIUS_X * Math.cos(angle),
			y: 50 + RADIUS_Y * Math.sin(angle),
			state
		};
	});

	return { nodes, height, nodeShare: nodeShare(nodes, height) };
}

function byId(a: { id: string }, b: { id: string }): number {
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
export function configuredProviders(providers: Provider[]): { id: string; name: string }[] {
	return providers
		.filter((provider) => provider.endpoint_count > 0 || provider.no_auth)
		.map((provider) => ({ id: provider.id, name: provider.name }))
		.sort(byId);
}

/**
 * The registry's ids mapped to their display names, lowercased on the id.
 *
 * The breakdown table resolves a provider group key through this (draft 014 F1): the key is the id the API
 * grouped by, and `openai` is a name the operator has to translate. The full read is used rather than
 * `configuredProviders`, because a provider that has usage and no endpoint still has a name.
 */
export function providerNameMap(providers: { id: string; name: string }[]): Map<string, string> {
	return new Map(providers.map((provider) => [provider.id.toLowerCase(), provider.name]));
}
