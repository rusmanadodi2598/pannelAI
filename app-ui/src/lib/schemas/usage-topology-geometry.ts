// The live drawing's geometry, including the two bands a node sits on.
//
// Split from `usage-topology-view.ts` because the two answer different questions: this file is arithmetic
// over a box (where a band's nodes land, how tall the box grows, how wide a label may be) and that file is
// which node carries which state. Pure both, so both are testable without a DOM, a clock, or a socket. The
// beam along a lit edge is a third question, and it lives in `usage-beam.ts`.

// The ellipse the two bands share, as a percentage of the drawing box. Percentages rather than pixels so the
// same layout fills a phone and a desktop panel without measuring anything, and the edge drawing uses the
// same numbers, so a line always ends where its node is.
//
// `RADIUS_X` is capped at 40 because a node at the widest share is 20% of the box and centre-anchored, so
// its centre must stay 10% clear of each edge. `RADIUS_Y` costs nothing in that arithmetic and is spent on
// separation instead: it puts the topmost combo 12% of the box below the client terminal and the mirrored
// pair at the bands' ends 14% apart, both more than one node row at the box's shortest height.
const RADIUS_X = 40;
const RADIUS_Y = 34;

// How far each band stays away from the horizontal axis, in degrees. Two nodes mirrored across that axis are
// `2 * RADIUS_Y * sin(edge)` apart vertically, and the row they would collide on is `NODE_HEIGHT /
// MIN_HEIGHT` of the box, 8.3%. Twelve degrees doubles that, so the margin survives whoever moves either
// constant later. What it protects against is not cosmetic: two nodes with the same x and a shared row make
// `nodeShare` return zero, and a share of zero is a drawing whose every box has collapsed to nothing.
const BAND_EDGE_DEGREES = 12;

// The box grows with the node count so labels do not collide, bounded at both ends: below the floor the
// drawing is a strip, and above the ceiling it is taller than the screen it sits on. The count is every node
// on both bands, because both bands need the room, and the floor is taller than the single ellipse needed
// because the terminals sit outside both bands rather than among them.
const MIN_HEIGHT = 360;
const MAX_HEIGHT = 640;
const HEIGHT_PER_NODE = 22;
const HEIGHT_BASE = 120;

// The node box at its largest, in the pixels the drawing's own metrics are written in: the widest node the
// panel draws (padding 8 + dot 8 + gap 8 + label 96 + border 2), and the node height the reference fork lays
// its own nodes out at (`nodeH = 30` in `ProviderTopology.js`).
//
// Those five numbers live in `UsageTopologyDrawing.svelte` and this constant lives here, so
// tests/schemas/usage-topology-geometry.test.ts adds them up from the drawing's own class list and compares
// the total to this value: a metric that moves without the cap moving fails there rather than quietly
// letting the drawing scale past the width a node ever reaches.
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

/** The client terminal, on the vertical axis the bands keep clear. */
export const CLIENT_POSITION = { x: 50, y: 4 };
/** The response terminal, at the other end of that axis. */
export const RESPONSE_POSITION = { x: 50, y: 96 };
/** The gateway's own position, which is the centre the bands are drawn around. */
export const GATEWAY_POSITION = { x: 50, y: 50 };

export type Point = { x: number; y: number };

/**
 * The drawing box's height in pixels, from every node it has to hold.
 */
export function boxHeight(nodeCount: number): number {
	return Math.min(MAX_HEIGHT, Math.max(MIN_HEIGHT, HEIGHT_BASE + HEIGHT_PER_NODE * nodeCount));
}

/**
 * The share of the drawing box's width one node may take, so that two nodes on one row never touch.
 *
 * Only pairs that share a row can collide whatever their horizontal distance, so the row is what this looks
 * at: a pair whose vertical distance is a whole node height apart is clear at any width. The result is a
 * share of the box's width, which is unitless on purpose: it holds at every width the box can take, and the
 * drawing caps it at `NODE_MAX_WIDTH` pixels once the box is wide enough.
 */
export function nodeShare(positions: Point[], height: number): number {
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
 * `count` positions across one band, left to right, each in the middle of its own slice.
 *
 * Centring a node in its slice rather than starting the slice at the band's edge is what keeps a band of one
 * node on the vertical axis and a band of two symmetric about it. `above` selects the combo band; the
 * provider band is its mirror image below the gateway.
 */
export function bandPositions(count: number, above: boolean): Point[] {
	if (count === 0) return [];
	const span = 180 - 2 * BAND_EDGE_DEGREES;
	const slice = span / count;

	return Array.from({ length: count }, (_unused, index) => {
		const fromEdge = BAND_EDGE_DEGREES + slice * (index + 0.5);
		const degrees = above ? -180 + fromEdge : 180 - fromEdge;
		const angle = (degrees * Math.PI) / 180;
		return { x: 50 + RADIUS_X * Math.cos(angle), y: 50 + RADIUS_Y * Math.sin(angle) };
	});
}
