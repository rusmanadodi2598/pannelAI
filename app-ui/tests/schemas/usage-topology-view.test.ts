// Live drawing derivation tests (src/lib/schemas/usage-topology-view.ts).
//
// The layout rows pin the arithmetic (the two bands, the box height, the state precedence, the terminals)
// because the drawing itself holds none of it. Which entities get a node at all is the other file's subject,
// in `usage-topology-nodeset.test.ts`.
//
// The positions below are measured against the drawing's own constants, not rounded for convenience: a band
// that moved by a third of a degree would change the collision answer, and that is the failure worth
// catching here rather than a label landing a pixel to the left.

import { describe, expect, it } from 'vitest';
import { topologyNodes } from '$lib/schemas/usage-topology-view';
import { forEachCase } from '../support/tables';
import { COMBOS, PROVIDERS, TOPOLOGY_IDLE } from '../support/topology-harness';
function many(count: number, prefix = 'p'): { id: string; name: string }[] {
	return Array.from({ length: count }, (_unused, index) => ({
		id: `${prefix}${index}`,
		name: `Node ${index}`
	}));
}

describe('topologyNodes', () => {
	it('draws nothing for a gateway with no provider and no combo', () => {
		const layout = topologyNodes([], [], TOPOLOGY_IDLE);

		expect(layout.nodes).toEqual([]);
		expect(layout.height).toBe(360);
	});

	it('puts one upstream below the gateway and one combo above it', () => {
		const layout = topologyNodes([{ id: 'openai', name: 'OpenAI' }], COMBOS, TOPOLOGY_IDLE);

		expect(layout.providers[0]).toMatchObject({ x: 50, y: 84 });
		expect(layout.combos[0]).toMatchObject({ x: 50, y: 16 });
	});

	it('puts two nodes of a band symmetric about the vertical axis', () => {
		const layout = topologyNodes(PROVIDERS, PROVIDERS, TOPOLOGY_IDLE);

		expect(layout.providers[0].x).toBeCloseTo(24.827, 3);
		expect(layout.providers[1].x).toBeCloseTo(75.173, 3);
		expect(layout.providers[0].y).toBeCloseTo(76.423, 3);
		expect(layout.combos[0].y).toBeCloseTo(23.577, 3);
		expect(layout.combos[1].y).toBeCloseTo(23.577, 3);
	});

	it('keeps every position inside the drawing box', () => {
		const layout = topologyNodes(many(24, 'pr'), many(8, 'cb'), TOPOLOGY_IDLE);

		for (const node of layout.nodes) {
			expect(node.x).toBeGreaterThan(0);
			expect(node.x).toBeLessThan(100);
			expect(node.y).toBeGreaterThan(0);
			expect(node.y).toBeLessThan(100);
		}
	});

	const heightCases = [
		{ name: 'an empty gateway', providers: 0, combos: 0, height: 360 },
		{ name: 'a gateway at the floor', providers: 5, combos: 5, height: 360 },
		{ name: 'a gateway that has grown past the floor', providers: 7, combos: 5, height: 384 },
		{ name: 'a gateway at the ceiling', providers: 25, combos: 8, height: 640 }
	];

	forEachCase(heightCases, (testCase) => {
		const layout = topologyNodes(
			many(testCase.providers, 'pr'),
			many(testCase.combos, 'cb'),
			TOPOLOGY_IDLE
		);

		expect(layout.height).toBe(testCase.height);
	});

	const stateCases = [
		{ name: 'no live state at all', live: TOPOLOGY_IDLE, state: 'idle' },
		{
			name: 'the provider of the last request',
			live: { ...TOPOLOGY_IDLE, last: 'openai' },
			state: 'last'
		},
		{
			name: 'the provider the gateway reported an error for',
			live: { ...TOPOLOGY_IDLE, error: 'openai' },
			state: 'error'
		},
		{
			name: 'a provider in flight',
			live: { ...TOPOLOGY_IDLE, active: ['openai'] },
			state: 'active'
		},
		{
			name: 'a provider in flight that also errored last',
			live: { ...TOPOLOGY_IDLE, active: ['openai'], error: 'openai' },
			state: 'active'
		},
		{
			name: 'a provider that is both last and in error',
			live: { ...TOPOLOGY_IDLE, last: 'openai', error: 'openai' },
			state: 'error'
		},
		{
			name: 'an id whose case differs from the registry spelling',
			live: { ...TOPOLOGY_IDLE, active: ['OpenAI'] },
			state: 'active'
		}
	];

	forEachCase(stateCases, (testCase) => {
		const layout = topologyNodes(PROVIDERS, [], testCase.live);

		expect(layout.nodes[0].state).toBe(testCase.state);
		expect(layout.nodes[1].state).toBe('idle');
	});

	it('lights a combo only from the combo names the frame carries, compared exactly', () => {
		const layout = topologyNodes([], COMBOS, { ...TOPOLOGY_IDLE, activeCombos: ['pro-tier'] });
		const unlit = topologyNodes([], COMBOS, { ...TOPOLOGY_IDLE, activeCombos: ['Pro-Tier'] });

		expect(layout.combos[0].state).toBe('active');
		expect(unlit.combos[0].state).toBe('idle');
	});

	it('never marks a combo as the last provider or an errored one', () => {
		// `last` and `error_provider` name providers on the wire. Reading them at a combo would light a
		// combo the gateway never said anything about.
		const layout = topologyNodes([], COMBOS, {
			...TOPOLOGY_IDLE,
			last: 'pro-tier',
			error: 'pro-tier'
		});

		expect(layout.combos[0].state).toBe('idle');
	});

	it('keys a combo apart from a provider of the same name', () => {
		// A combo is addressed by a name the operator typed, and `openai` is a legal one. Bare identities
		// would put two nodes under one key and light the wrong stage of the path.
		const layout = topologyNodes(
			[{ id: 'openai', name: 'OpenAI' }],
			[{ id: 'openai', name: 'openai' }],
			TOPOLOGY_IDLE
		);

		expect(layout.providers[0].key).toBe('provider:openai');
		expect(layout.combos[0].key).toBe('combo:openai');
		expect(layout.nodes.map((node) => node.key)).toEqual(['provider:openai', 'combo:openai']);
	});

	it('keeps the node ids as the registry and the combo list spell them', () => {
		const layout = topologyNodes(PROVIDERS, COMBOS, TOPOLOGY_IDLE);

		expect(layout.providers.map((node) => node.id)).toEqual(['openai', 'anthropic']);
		expect(layout.combos.map((node) => node.id)).toEqual(['pro-tier']);
	});
	it('gives a node a fifth of the box while the bands still leave room between neighbours', () => {
		for (const count of [1, 2, 3, 4, 5]) {
			const layout = topologyNodes(many(count, 'pr'), many(count, 'cb'), TOPOLOGY_IDLE);

			expect(layout.nodeShare, `${count} per band`).toBeGreaterThan(0.15);
		}
	});

	it('shrinks the share once a fifth of the box would leave two neighbours touching', () => {
		const five = topologyNodes(many(5, 'pr'), many(5, 'cb'), TOPOLOGY_IDLE).nodeShare;
		const eight = topologyNodes(many(8, 'pr'), many(8, 'cb'), TOPOLOGY_IDLE).nodeShare;
		const twelve = topologyNodes(many(12, 'pr'), many(12, 'cb'), TOPOLOGY_IDLE).nodeShare;

		expect(five).toBeLessThan(0.2);
		expect(eight).toBeLessThan(five);
		expect(twelve).toBeLessThan(eight);
	});

	it('never returns a share of zero, at any count it is given', () => {
		// The claim is narrow and it is the one that matters: a zero share makes the drawing's own unit zero,
		// and every box on it (the nodes, the gateway, the beam strokes) collapses to nothing. A pair of
		// nodes mirrored across the horizontal axis is how that happens, because they share an x and only
		// their vertical distance keeps them on different rows.
		for (let providers = 0; providers <= 24; providers += 1) {
			for (let combos = 0; combos <= 8; combos += 1) {
				const layout = topologyNodes(many(providers, 'pr'), many(combos, 'cb'), TOPOLOGY_IDLE);

				expect(layout.nodeShare, `${providers} providers and ${combos} combos`).toBeGreaterThan(0);
			}
		}
	});

	it('keeps every pair of nodes apart at every box width, for every count it is given', () => {
		// The drawing's own rule, modelled here: a node is at most `min(share * boxWidth, 130)` wide and
		// 30px tall at that cap, shrinking with the same unit, so its height is 30 * width / 130. The claim it
		// pins: no two node boxes intersect, at any width.
		for (const width of [180, 228, 320, 480, 650, 900, 1086]) {
			for (let count = 2; count <= 24; count += 1) {
				const layout = topologyNodes(many(count, 'pr'), many(count, 'cb'), TOPOLOGY_IDLE);
				const unit = Math.min(1, (layout.nodeShare * width) / 130);
				const nodeWidth = 130 * unit;
				const nodeHeight = 30 * unit;

				for (let i = 0; i < layout.nodes.length; i += 1) {
					for (let j = i + 1; j < layout.nodes.length; j += 1) {
						const dx = (Math.abs(layout.nodes[i].x - layout.nodes[j].x) / 100) * width;
						const dy = (Math.abs(layout.nodes[i].y - layout.nodes[j].y) / 100) * layout.height;

						expect(
							dx >= nodeWidth || dy >= nodeHeight,
							`${count} per band at ${width}px: nodes ${i} and ${j} intersect`
						).toBe(true);
					}
				}
			}
		}
	});
});
