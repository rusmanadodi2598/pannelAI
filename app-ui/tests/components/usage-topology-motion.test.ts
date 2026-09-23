// Motion rows for the live drawing (src/lib/components/UsageTopology.svelte and its box,
// UsageTopologyDrawing.svelte; draft 013 F1/F2 and draft 015 F1).
//
// The drawing's motion is a state indicator with two conditions: the frame names the provider, and frames
// are arriving. These rows pin both, plus the off switches: reduced motion for the reader, and a stream
// that is paused or down for the panel. What has to survive the motion stopping is the state itself, which
// is why one row asserts colour with the stream off.
//
// The beam's own parts and numbers are measured next door in `usage-topology-beam.test.ts`; what is left
// here is when anything moves, and what the gateway and the node do while it does.

import { cleanup, screen, within } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { beamAt, draw, edgeFor, entry, GLOW, gateway, nodeFor } from '../support/topology-harness';

vi.mock('$app/paths', () => ({ resolve: (path: string) => path }));

afterEach(() => {
	cleanup();
});

describe('UsageTopology motion', () => {
	it('stops every moving part when the stream is not live, and keeps the state it last saw', () => {
		const container = draw({ active: [entry('openai')], live: false });
		const edge = edgeFor(container, 'OpenAI');
		const node = nodeFor(container, 'OpenAI');

		expect(beamAt(container, 'OpenAI', 'core')).toHaveLength(0);
		expect(edge.classList.contains('stroke-[var(--color-ok)]')).toBe(true);
		expect(node.querySelector('.animate-ping')).toBeNull();
		expect(node.classList.contains('border-[var(--color-ok)]')).toBe(true);
		expect(within(node).getByText('OpenAI').classList.contains('text-[var(--color-ok)]')).toBe(
			true
		);
	});

	it('lets a reader who asked for less motion stop the beam and drop the dots', () => {
		const container = draw({ active: [entry('openai')] });

		expect(
			beamAt(container, 'OpenAI', 'core')[0].classList.contains('motion-reduce:animate-none')
		).toBe(true);
		expect(
			beamAt(container, 'OpenAI', 'halo')[0].classList.contains('motion-reduce:animate-none')
		).toBe(true);
		expect(beamAt(container, 'OpenAI', 'orb')[0].classList.contains('motion-reduce:hidden')).toBe(
			true
		);
		expect(beamAt(container, 'OpenAI', 'spark')[0].classList.contains('motion-reduce:hidden')).toBe(
			true
		);
	});

	it('marks the routing node with the status colour and a glow, and no other node', () => {
		const container = draw({ active: [entry('openai')] });
		const routing = nodeFor(container, 'OpenAI');
		const idle = nodeFor(container, 'Anthropic');

		expect(routing.classList.contains('border-[var(--color-ok)]')).toBe(true);
		expect(routing.classList.contains(GLOW)).toBe(true);
		expect(idle.classList.contains('border-[var(--color-ok)]')).toBe(false);
		expect(idle.classList.contains(GLOW)).toBe(false);
	});

	it('pulses the gateway while it is routing, and stops when the stream does', () => {
		draw({ active: [entry('openai')] });

		expect(gateway().classList.contains('animate-router-pulse')).toBe(true);
		expect(gateway().classList.contains('border-[var(--color-ok)]')).toBe(true);

		cleanup();
		draw({ active: [entry('openai')], live: false });

		expect(gateway().classList.contains('animate-router-pulse')).toBe(false);
		expect(gateway().classList.contains('border-[var(--color-ok)]')).toBe(true);
	});

	it('shakes the gateway mark and flickers its label only while it is routing', () => {
		const routing = draw({ active: [entry('openai')] });

		expect(routing.querySelector('img')?.classList.contains('animate-router-shake')).toBe(true);
		expect(
			within(gateway()).getByText('Gateway').classList.contains('animate-router-flicker')
		).toBe(true);

		cleanup();
		const idle = draw();

		expect(idle.querySelector('img')?.classList.contains('animate-router-shake')).toBe(false);
		expect(
			within(gateway()).getByText('Gateway').classList.contains('animate-router-flicker')
		).toBe(false);
	});

	it('counts the requests in flight at the gateway, not the nodes that carry them', () => {
		draw({ active: [entry('openai'), entry('openai'), entry('anthropic')] });

		expect(within(gateway()).getByText('3')).toBeTruthy();
	});

	it('shows no count on the gateway when nothing is in flight', () => {
		draw();

		expect(within(gateway()).queryByText('0')).toBeNull();
	});

	it('sizes the gateway from the drawing unit too, so a narrow box cannot overlap it', () => {
		// The gateway is a box like a node: at 390px it was 108px wide in a 228px drawing and covered part of
		// the node beside it, so it takes the same unit (draft 018 F1).
		draw();

		const card = gateway();

		expect(card.classList.contains('px-[calc(12px*var(--u))]')).toBe(true);
		expect(card.classList.contains('gap-[calc(8px*var(--u))]')).toBe(true);
		expect(card.classList.contains('[font-size:calc(14px*var(--u))]')).toBe(true);
		expect(card.querySelector('img')?.classList.contains('size-[calc(16px*var(--u))]')).toBe(true);
	});

	it('says in words what the drawing moves, since the drawing is hidden from assistive technology', () => {
		draw();

		expect(
			screen.getByText(/carries a beam with dots running along it, its node a soft glow/)
		).toBeTruthy();
	});
});
