// The live drawing's derivations (docs/DRAFT/012-USAGE-LIVE-UI-READINESS.md F3, and the request path the
// owner asked for on 2026-10-03, drafted in docs/DRAFT/043-USAGE-COMBO-NODE-FLOW.md).
//
// Three files answer three questions about the same picture: which entities get a node at all
// (`usage-topology-nodeset.ts`), where a node goes and how wide it may be
// (`usage-topology-geometry.ts`), and — here — which state each node and terminal carries. All three are
// pure, so the drawing itself holds no arithmetic and none of this needs a DOM, a clock, or a socket.
//
// The drawing is a request path, not a ring: `Client >> Combo >> Gateway >> Upstream >> Response`. The
// gateway stays at the centre, the upstreams it calls sit on the band below it, the combos a client
// addressed sit on the band above, and the two terminals sit on the vertical axis. Combos and upstreams are
// kept in disjoint bands because two nodes mirrored across the horizontal axis share an x, and x is the only
// number the share has to fall back on: a shared row with no horizontal gap is a share of zero, and a share
// of zero is a drawing whose every box has collapsed to nothing.

import { bandPositions, boxHeight, nodeShare } from './usage-topology-geometry';

export type TopologyState = 'active' | 'last' | 'error' | 'idle';

/** Which stage of the request path a node is: an upstream the gateway calls, or a combo a client named. */
export type TopologyKind = 'provider' | 'combo';

export type TopologyNode = {
	/** The identity the wire carries: a provider id, or a combo name. */
	id: string;
	/**
	 * The drawing's own key, namespaced by kind.
	 *
	 * A combo is addressed by a name the operator typed, and a combo literally named `openai` is legal.
	 * Keying both kinds on their bare identity would put two nodes under one key and light the wrong one.
	 */
	key: string;
	kind: TopologyKind;
	name: string;
	/** Position as a percentage of the drawing box, so the drawing needs no measurement to place it. */
	x: number;
	y: number;
	state: TopologyState;
};

/**
 * A terminal: one end of the path, drawn whether or not anything is flowing through it.
 *
 * Terminals are not nodes. They carry no identity from the wire, so they stay out of `nodes` — which is what
 * keeps the share from treating a fixed position as a data node's, and what keeps the drawing's tests from
 * finding them by label the way they find a provider.
 */
export type TopologyTerminal = {
	state: TopologyState;
};

export type TopologyLayout = {
	/** Every node, both bands, in the order the drawing paints them. */
	nodes: TopologyNode[];
	providers: TopologyNode[];
	combos: TopologyNode[];
	client: TopologyTerminal;
	response: TopologyTerminal;
	/**
	 * The hop from the client straight to the gateway.
	 *
	 * It is lit only by requests that addressed no combo, because a combo request travels
	 * client → combo → gateway and a beam on the direct hop would claim a request that never took it.
	 */
	direct: TopologyTerminal;
	/** The drawing box's height in pixels, from the node count. */
	height: number;
	/** The width one node may take, as a share of the drawing box's width (draft 018). */
	nodeShare: number;
};

/** What the stream says is happening, in the form the layout needs rather than the form the wire sends. */
export type TopologyLive = {
	/** The provider ids with a request in flight. */
	active: string[];
	/** The combo names with a request in flight. */
	activeCombos: string[];
	/** The requests in flight that addressed no combo, which is what the direct hop carries. */
	directCount: number;
	/** The provider of the most recent completed request, or an empty string. */
	last: string;
	/** The provider the gateway last reported an error for, or an empty string. */
	error: string;
};

function lower(value: string): string {
	return value.toLowerCase();
}

function providerState(
	id: string,
	active: Set<string>,
	error: string,
	last: string
): TopologyState {
	const key = lower(id);
	if (active.has(key)) return 'active';
	if (key === error) return 'error';
	if (key === last) return 'last';
	return 'idle';
}

/**
 * The drawing: the upstreams the gateway calls below it, the combos a client addressed above it, and the two
 * terminals the request enters and leaves through.
 *
 * Provider states are exclusive and take the reference fork's precedence: a provider with a request in
 * flight is active whatever else is true of it, then an error outranks the last-request mark, because "the
 * gateway reported an error here" is the more useful fact to have on screen.
 *
 * Provider ids are compared without case, because the frame's `error_provider` and the registry's ids are
 * both strings from a gateway the panel does not control, and a difference of case is not a difference of
 * provider. Combo names are compared exactly: a name is an identifier the gateway wrote back verbatim, and
 * folding its case would light one combo for a request that addressed another whose name differs only in
 * case.
 */
export function topologyNodes(
	providers: { id: string; name: string }[],
	combos: { id: string; name: string }[],
	live: TopologyLive
): TopologyLayout {
	const height = boxHeight(providers.length + combos.length);
	const active = new Set(live.active.map(lower));
	const activeCombos = new Set(live.activeCombos);
	const last = lower(live.last);
	const error = lower(live.error);
	const inFlight = live.active.length;

	const providerSpots = bandPositions(providers.length, false);
	const comboSpots = bandPositions(combos.length, true);

	const providerNodes = providers.map<TopologyNode>((provider, index) => ({
		id: provider.id,
		key: `provider:${provider.id}`,
		kind: 'provider',
		name: provider.name,
		...providerSpots[index],
		state: providerState(provider.id, active, error, last)
	}));

	const comboNodes = combos.map<TopologyNode>((combo, index) => ({
		id: combo.id,
		key: `combo:${combo.id}`,
		kind: 'combo',
		name: combo.name,
		...comboSpots[index],
		state: activeCombos.has(combo.id) ? 'active' : 'idle'
	}));

	const nodes = [...providerNodes, ...comboNodes];

	return {
		nodes,
		providers: providerNodes,
		combos: comboNodes,
		client: { state: inFlight > 0 ? 'active' : 'idle' },
		direct: { state: live.directCount > 0 ? 'active' : 'idle' },
		response: { state: inFlight > 0 ? 'active' : last === '' ? 'idle' : 'last' },
		height,
		nodeShare: nodeShare(nodes, height)
	};
}
