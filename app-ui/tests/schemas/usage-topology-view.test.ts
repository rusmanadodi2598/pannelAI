// Live drawing derivation tests (src/lib/schemas/usage-topology-view.ts, draft 012 F3).
//
// Two subjects: the layout the drawing is placed from, and the node set it is given. The layout rows pin
// the arithmetic (the ellipse, the box height, the state precedence) because the drawing itself holds none
// of it, and the node-set rows pin the difference between a provider the gateway knows and one it could
// route anything through.

import { describe, expect, it } from 'vitest';
import type { Provider } from '$lib/schemas/provider';
import { configuredProviders, topologyNodes } from '$lib/schemas/usage-topology-view';
import { forEachCase } from '../support/tables';

const PROVIDERS = [
	{ id: 'openai', name: 'OpenAI' },
	{ id: 'anthropic', name: 'Anthropic' }
];

describe('topologyNodes', () => {
	it('draws nothing for a gateway with no configured provider', () => {
		const layout = topologyNodes([], { active: [], last: '', error: '' });

		expect(layout.nodes).toEqual([]);
		expect(layout.height).toBe(320);
	});

	it('puts the first node at the top of the ellipse and the second opposite it', () => {
		const layout = topologyNodes(PROVIDERS, { active: [], last: '', error: '' });

		expect(layout.nodes[0].x).toBeCloseTo(50, 6);
		expect(layout.nodes[0].y).toBeCloseTo(12, 6);
		expect(layout.nodes[1].x).toBeCloseTo(50, 6);
		expect(layout.nodes[1].y).toBeCloseTo(88, 6);
	});

	it('keeps every position inside the drawing box', () => {
		const many = Array.from({ length: 24 }, (_unused, index) => ({
			id: `p${index}`,
			name: `Provider ${index}`
		}));

		for (const node of topologyNodes(many, { active: [], last: '', error: '' }).nodes) {
			expect(node.x).toBeGreaterThan(0);
			expect(node.x).toBeLessThan(100);
			expect(node.y).toBeGreaterThan(0);
			expect(node.y).toBeLessThan(100);
		}
	});

	const heightCases = [
		{ name: 'a gateway with one provider', count: 1, height: 320 },
		{ name: 'a gateway at the floor', count: 5, height: 320 },
		{ name: 'a gateway that has grown past the floor', count: 10, height: 340 },
		{ name: 'a gateway at the ceiling', count: 25, height: 640 },
		{ name: 'a gateway past the ceiling', count: 40, height: 640 }
	];

	forEachCase(heightCases, (testCase) => {
		const many = Array.from({ length: testCase.count }, (_unused, index) => ({
			id: `p${index}`,
			name: `Provider ${index}`
		}));

		expect(topologyNodes(many, { active: [], last: '', error: '' }).height).toBe(testCase.height);
	});

	const stateCases = [
		{ name: 'no live state at all', live: { active: [], last: '', error: '' }, state: 'idle' },
		{
			name: 'the provider of the last request',
			live: { active: [], last: 'openai', error: '' },
			state: 'last'
		},
		{
			name: 'the provider the gateway reported an error for',
			live: { active: [], last: '', error: 'openai' },
			state: 'error'
		},
		{
			name: 'a provider in flight',
			live: { active: ['openai'], last: '', error: '' },
			state: 'active'
		},
		{
			name: 'a provider in flight that also errored last',
			live: { active: ['openai'], last: '', error: 'openai' },
			state: 'active'
		},
		{
			name: 'a provider that is both last and in error',
			live: { active: [], last: 'openai', error: 'openai' },
			state: 'error'
		},
		{
			name: 'an id whose case differs from the registry spelling',
			live: { active: ['OpenAI'], last: '', error: '' },
			state: 'active'
		}
	];

	forEachCase(stateCases, (testCase) => {
		const layout = topologyNodes(PROVIDERS, testCase.live);

		expect(layout.nodes[0].state).toBe(testCase.state);
		expect(layout.nodes[1].state).toBe('idle');
	});

	it('keeps the node ids as the registry spells them', () => {
		expect(
			topologyNodes(PROVIDERS, { active: [], last: '', error: '' }).nodes.map((node) => node.id)
		).toEqual(['openai', 'anthropic']);
	});

	function many(count: number) {
		return Array.from({ length: count }, (_unused, index) => ({
			id: `p${index}`,
			name: `Provider ${index}`
		}));
	}

	it('gives a node a fifth of the box while a fifth still leaves room between neighbours', () => {
		// Up to nine providers the tightest pair that shares a row is more than a fifth of the box apart, so
		// the cap is what the drawing uses. Ten is where it stops: measured, that pair is 23.5% apart and the
		// share drops to 0.19985 (draft 018 F1).
		for (const count of [1, 2, 3, 6, 7, 9]) {
			expect(topologyNodes(many(count), { active: [], last: '', error: '' }).nodeShare).toBeCloseTo(
				0.2,
				6
			);
		}
	});

	it('shrinks the share once a fifth of the box would leave two neighbours touching', () => {
		// Eleven providers is where it is visible: the node at the top and its neighbour are 21.6% of the box
		// apart horizontally and only 6% apart vertically, which is closer than one node is tall.
		const eleven = topologyNodes(many(11), { active: [], last: '', error: '' }).nodeShare;
		const twelve = topologyNodes(many(12), { active: [], last: '', error: '' }).nodeShare;

		expect(eleven).toBeLessThan(0.2);
		expect(eleven).toBeGreaterThan(0.18);
		expect(twelve).toBeLessThan(eleven);
	});

	it('keeps every pair of nodes apart at every box width, for every count it is given', () => {
		// The drawing's own rule, modelled here: a node is at most `min(share * boxWidth, 130)` wide and
		// 30px tall at that cap, shrinking with the same unit, so its height is 30 * width / 130. The claim
		// is the one draft 016 F8 measured as false at 390px: no two node boxes intersect, at any width.
		for (const width of [180, 228, 320, 480, 650, 900, 1086]) {
			for (let count = 2; count <= 40; count += 1) {
				const layout = topologyNodes(many(count), { active: [], last: '', error: '' });
				const unit = Math.min(1, (layout.nodeShare * width) / 130);
				const nodeWidth = 130 * unit;
				const nodeHeight = 30 * unit;

				for (let i = 0; i < layout.nodes.length; i += 1) {
					for (let j = i + 1; j < layout.nodes.length; j += 1) {
						const dx = (Math.abs(layout.nodes[i].x - layout.nodes[j].x) / 100) * width;
						const dy = (Math.abs(layout.nodes[i].y - layout.nodes[j].y) / 100) * layout.height;

						expect(
							dx >= nodeWidth || dy >= nodeHeight,
							`${count} nodes at ${width}px: nodes ${i} and ${j} intersect`
						).toBe(true);
					}
				}
			}
		}
	});
});

describe('configuredProviders', () => {
	function provider(overrides: Partial<Provider> & { id: string }): Provider {
		return {
			name: overrides.id,
			category: 'apikey',
			auth_type: 'api_key',
			auth_modes: [],
			has_oauth: false,
			no_auth: false,
			routability: 'routable',
			endpoint_count: 0,
			status_summary: { total: 0, active: 0, disabled: 0, error: 0, rate_limited: 0 },
			...overrides
		};
	}

	it('keeps a provider with a stored endpoint', () => {
		const kept = configuredProviders([provider({ id: 'openai', endpoint_count: 2 })]);

		expect(kept).toEqual([{ id: 'openai', name: 'openai' }]);
	});

	it('drops a provider nothing has been configured for', () => {
		expect(configuredProviders([provider({ id: 'openai' })])).toEqual([]);
	});

	it('keeps a provider that needs no credential, because its endpoint count stays at zero', () => {
		const kept = configuredProviders([provider({ id: 'opencode', no_auth: true })]);

		expect(kept).toEqual([{ id: 'opencode', name: 'opencode' }]);
	});

	it('sorts by id so the drawing is the same drawing on every read', () => {
		const kept = configuredProviders([
			provider({ id: 'openai', endpoint_count: 1 }),
			provider({ id: 'anthropic', endpoint_count: 1 }),
			provider({ id: 'zai', no_auth: true })
		]);

		expect(kept.map((entry) => entry.id)).toEqual(['anthropic', 'openai', 'zai']);
	});
});
