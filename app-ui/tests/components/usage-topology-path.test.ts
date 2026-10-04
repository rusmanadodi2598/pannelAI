// The live drawing's request path (src/lib/components/UsageTopologyEdges.svelte and
// UsageTopologyHop.svelte, docs/DRAFT/043-USAGE-COMBO-NODE-FLOW.md F1/F2).
//
// Split from `usage-topology.test.ts` on the seam the drafts already use: that file is about which node
// carries which state, this one is about the path the nodes form — `Client >> Combo >> Gateway >> Upstream
// >> Response` — and about the hops between the stages. The rows are about which hops exist and which of
// them a frame lights, because a hop is drawn eleven lines when it is carrying a beam and one when it is
// not, so counting strokes would be counting the wrong thing.

import { cleanup, render, within } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import UsageTopology from '../../src/lib/components/UsageTopology.svelte';
import type { UsageLiveActive } from '$lib/schemas/usage-live';

vi.mock('$app/paths', () => ({ resolve: (path: string) => path }));

const PROVIDERS = [
	{ id: 'openai', name: 'OpenAI' },
	{ id: 'anthropic', name: 'Anthropic' }
];
const COMBOS = [
	{ id: 'pro-tier', name: 'pro-tier' },
	{ id: 'fast', name: 'fast' }
];

function entry(providerId: string, combo?: string): UsageLiveActive {
	return {
		provider_id: providerId,
		started_at: '2026-09-22T10:00:00Z',
		...(combo === undefined ? {} : { combo })
	};
}

type Props = {
	providers: { id: string; name: string }[];
	combos: { id: string; name: string }[];
	active: UsageLiveActive[];
	last: string;
	error: string;
	live: boolean;
};

function draw(overrides: Partial<Props> = {}): HTMLElement {
	const props: Props = {
		providers: PROVIDERS,
		combos: [],
		active: [],
		last: '',
		error: '',
		live: true,
		...overrides
	};
	return render(UsageTopology, { props }).container;
}

function nodeFor(container: HTMLElement, name: string): HTMLElement {
	return within(container).getByText(name).parentElement as HTMLElement;
}

/** The lines of one hop of the path. */
function edgesFor(container: HTMLElement, hop: string): SVGLineElement[] {
	return [...container.querySelectorAll<SVGLineElement>(`line[data-edge="${hop}"]`)];
}

/** The lines of one hop drawn as a single edge, which is every hop that is not routing. */
function plainEdges(container: HTMLElement, hop: string): SVGLineElement[] {
	return edgesFor(container, hop).filter((line) => line.getAttribute('data-beam') === null);
}

/** The core stroke of every beam on one hop, which is the part that says a request is travelling it. */
function coreBeams(container: HTMLElement, hop: string): SVGLineElement[] {
	return edgesFor(container, hop).filter((line) => line.getAttribute('data-beam') === 'core');
}

afterEach(() => {
	cleanup();
});

describe('the path the drawing draws', () => {
	it('draws the two ends of the path even while nothing is moving', () => {
		const container = draw();

		expect(within(container).getByText('Client')).toBeTruthy();
		expect(within(container).getByText('Response')).toBeTruthy();
	});

	it('draws a node per combo, above the gateway and below the client', () => {
		const container = draw({ combos: COMBOS });

		// The band the drawing places them on is the claim: a combo sits between the client and the gateway,
		// an upstream below the gateway. Asserting the positions is what keeps the two bands from quietly
		// swapping places, which no label would show.
		expect(Number.parseFloat(nodeFor(container, 'pro-tier').style.top)).toBeLessThan(50);
		expect(Number.parseFloat(nodeFor(container, 'OpenAI').style.top)).toBeGreaterThan(50);
	});

	it('pulses the combo the frame names and no other combo', () => {
		const container = draw({ combos: COMBOS, active: [entry('openai', 'pro-tier')] });

		expect(nodeFor(container, 'pro-tier').querySelector('.animate-ping')).toBeTruthy();
		expect(nodeFor(container, 'fast').querySelector('.animate-ping')).toBeNull();
	});

	it('drops the combo band when no combo is configured', () => {
		const container = draw({ combos: [] });

		expect(edgesFor(container, 'client-combo')).toHaveLength(0);
		expect(edgesFor(container, 'combo-gateway')).toHaveLength(0);
	});

	it('carries one hop in and one hop out for every combo', () => {
		const container = draw({ combos: COMBOS });

		expect(plainEdges(container, 'client-combo')).toHaveLength(COMBOS.length);
		expect(plainEdges(container, 'combo-gateway')).toHaveLength(COMBOS.length);
	});

	it('lights the combo the frame names, on both of its hops, and no other combo', () => {
		const container = draw({ combos: COMBOS, active: [entry('openai', 'pro-tier')] });

		expect(coreBeams(container, 'client-combo')).toHaveLength(1);
		expect(coreBeams(container, 'combo-gateway')).toHaveLength(1);
		// `fast` keeps its plain line, so an unlit combo is still on the drawing rather than removed by
		// another combo's traffic.
		expect(plainEdges(container, 'client-combo')).toHaveLength(1);
	});

	it('leaves the direct hop plain when every request in flight came through a combo', () => {
		const container = draw({ combos: COMBOS, active: [entry('openai', 'pro-tier')] });

		// The combo's own two hops carry the beam; a beam on client → gateway would state a second request
		// that never took that hop.
		expect(coreBeams(container, 'client-gateway')).toHaveLength(0);
		expect(plainEdges(container, 'client-gateway')).toHaveLength(1);
		expect(coreBeams(container, 'client-combo')).toHaveLength(1);
	});

	it('draws the way back: one hop from every upstream to the response terminal', () => {
		const container = draw();

		expect(plainEdges(container, 'provider-response')).toHaveLength(PROVIDERS.length);
		expect(plainEdges(container, 'gateway-provider')).toHaveLength(PROVIDERS.length);
		expect(plainEdges(container, 'client-gateway')).toHaveLength(1);
	});

	it('travels the return hop outward from the upstream, so the answer reads as coming back', () => {
		const container = draw({ active: [entry('openai')] });
		const back = coreBeams(container, 'provider-response')[0];
		const forward = coreBeams(container, 'gateway-provider')[0];

		// Direction on this drawing is the endpoint order, not a second keyframe: a beam moves from a line's
		// start to its end. `gateway-provider` starts at the centre; the way back starts at the node.
		expect(forward.getAttribute('x1')).toBe('50');
		expect(Number.parseFloat(back.getAttribute('y2') ?? '')).toBeCloseTo(96, 6);
	});

	it('never gives one turbulence filter to two hops', () => {
		// An active upstream carries a beam on two hops at once. Keyed by the node alone, both would declare
		// the same id and the second would resolve to the first's filter.
		const container = draw({ active: [entry('openai')] });
		const filters = [...container.querySelectorAll('filter')].map((node) => node.id);
		const ids = filters.map((id) => `url(#${id})`);
		const used = [...container.querySelectorAll('line[filter]')].map(
			(line) => line.getAttribute('filter') as string
		);

		expect(new Set(filters).size).toBe(filters.length);
		expect(used.every((one) => ids.includes(one))).toBe(true);
	});

	it('keeps a combo that shares a provider id as two cards', () => {
		const container = draw({
			providers: [{ id: 'openai', name: 'openai' }],
			combos: [{ id: 'openai', name: 'openai' }]
		});

		// Two elements with the same text is what a name collision looks like here, and it is the honest
		// outcome: the combo and the upstream are two facts about one request, and the drawing says both.
		expect(within(container).getAllByText('openai')).toHaveLength(2);
	});
});
