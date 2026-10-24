// Which terminals the live drawing lights (src/lib/schemas/usage-topology-view.ts).
//
// Split from `usage-topology-view.test.ts`, which proves where the nodes sit and what state each wears.
// The terminals are the other half of the same sentence, and the one half an operator reads as "this is
// moving": the client side lights while a request is in flight, the response side lights on the way back
// and stays lit as the last fact, and the direct hop lights only for traffic that addressed no combo.

import { describe, expect, it } from 'vitest';
import { topologyNodes } from '$lib/schemas/usage-topology-view';
import { COMBOS, PROVIDERS, TOPOLOGY_IDLE } from '../support/topology-harness';

describe('the terminals the drawing lights', () => {
	it('lights the client terminal while anything is in flight', () => {
		const idle = topologyNodes(PROVIDERS, COMBOS, TOPOLOGY_IDLE);
		const busy = topologyNodes(PROVIDERS, COMBOS, { ...TOPOLOGY_IDLE, active: ['openai'] });

		expect(idle.client.state).toBe('idle');
		expect(busy.client.state).toBe('active');
	});

	it('lights the response terminal on the way back, and keeps it lit as the last fact after', () => {
		const busy = topologyNodes(PROVIDERS, COMBOS, { ...TOPOLOGY_IDLE, active: ['openai'] });
		const finished = topologyNodes(PROVIDERS, COMBOS, { ...TOPOLOGY_IDLE, last: 'openai' });

		expect(busy.response.state).toBe('active');
		expect(finished.response.state).toBe('last');
		expect(topologyNodes(PROVIDERS, COMBOS, TOPOLOGY_IDLE).response.state).toBe('idle');
	});

	it('lights the direct hop only for traffic that addressed no combo', () => {
		// A combo request travels client → combo → gateway. Beaming the direct line as well would state a
		// second request that never took that hop, so the line is lit by its own count rather than by the
		// client terminal's, which is honest about "something is in flight".
		const comboTraffic = topologyNodes(PROVIDERS, COMBOS, {
			...TOPOLOGY_IDLE,
			active: ['openai'],
			activeCombos: ['pro-tier'],
			directCount: 0
		});
		const plainTraffic = topologyNodes(PROVIDERS, COMBOS, {
			...TOPOLOGY_IDLE,
			active: ['openai'],
			directCount: 1
		});

		expect(comboTraffic.client.state).toBe('active');
		expect(comboTraffic.direct.state).toBe('idle');
		expect(plainTraffic.direct.state).toBe('active');
	});

	it('keeps the terminals out of the node list', () => {
		// They carry no identity from the wire, so treating one as a node would let a fixed position decide
		// how wide every label on the drawing may be.
		const layout = topologyNodes(PROVIDERS, COMBOS, TOPOLOGY_IDLE);

		expect(layout.nodes.map((node) => node.key)).toEqual([
			'provider:openai',
			'provider:anthropic',
			'combo:pro-tier'
		]);
	});
});
