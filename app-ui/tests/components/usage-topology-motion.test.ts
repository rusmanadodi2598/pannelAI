// Motion rows for the live drawing (src/lib/components/UsageTopology.svelte, draft 013 F1 and F2).
//
// The drawing's motion is a state indicator with two conditions: the frame names the provider, and frames
// are arriving. These rows pin both, plus the off switches: reduced motion for the reader, and a stream
// that is paused or down for the panel. What has to survive the motion stopping is the state itself, which
// is why the last rows assert colour and count with the stream off.

import { cleanup, render, screen, within } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import UsageTopology from '../../src/lib/components/UsageTopology.svelte';
import type { UsageLiveActive } from '$lib/schemas/usage-live';

vi.mock('$app/paths', () => ({ resolve: (path: string) => path }));

const PROVIDERS = [
	{ id: 'openai', name: 'OpenAI' },
	{ id: 'anthropic', name: 'Anthropic' }
];

const GLOW = 'shadow-[0_0_16px_color-mix(in_srgb,var(--color-ok)_25%,transparent)]';
const DASH = '[stroke-dasharray:3_3]';

function entry(providerId: string): UsageLiveActive {
	return { provider_id: providerId, started_at: '2026-09-22T10:00:00Z' };
}

type Props = {
	providers: { id: string; name: string }[];
	active: UsageLiveActive[];
	last: string;
	error: string;
	live: boolean;
};

function draw(overrides: Partial<Props> = {}): HTMLElement {
	const props: Props = {
		providers: PROVIDERS,
		active: [],
		last: '',
		error: '',
		live: true,
		...overrides
	};
	return render(UsageTopology, { props }).container;
}

/** The node box for a provider, which is the element its label sits in. */
function nodeFor(container: HTMLElement, name: string): HTMLElement {
	return within(container).getByText(name).parentElement as HTMLElement;
}

/**
 * The edge whose far end is a provider's node, matched by the position both are drawn at.
 *
 * Matching on position rather than on order is the point: the drawing's order is the provider list's
 * order, and a row about "only this provider's line moves" must not pass because two lists happen to
 * agree.
 */
function edgeFor(container: HTMLElement, name: string): SVGLineElement {
	const box = nodeFor(container, name);
	const x = Number.parseFloat(box.style.left);
	const y = Number.parseFloat(box.style.top);
	const match = [...container.querySelectorAll('line')].find(
		(line) =>
			Number.parseFloat(line.getAttribute('x2') ?? '') === x &&
			Number.parseFloat(line.getAttribute('y2') ?? '') === y
	);

	if (match === undefined) throw new Error(`no edge ends at the node for ${name}`);
	return match;
}

function gateway(): HTMLElement {
	return screen.getByText('Gateway').parentElement as HTMLElement;
}

afterEach(() => {
	cleanup();
});

describe('UsageTopology motion', () => {
	it('flows the line to the provider that is routing, and no other', () => {
		const container = draw({ active: [entry('openai')] });

		expect(edgeFor(container, 'OpenAI').classList.contains('animate-flow')).toBe(true);
		expect(edgeFor(container, 'Anthropic').classList.contains('animate-flow')).toBe(false);
	});

	it('carries the dash pattern on the routing line only, so a solid line means nothing is moving', () => {
		const container = draw({ active: [entry('openai')] });

		expect(edgeFor(container, 'OpenAI').classList.contains(DASH)).toBe(true);
		expect(edgeFor(container, 'Anthropic').classList.contains(DASH)).toBe(false);
	});

	it('lets a reader who asked for less motion stop the flow', () => {
		const container = draw({ active: [entry('openai')] });

		expect(edgeFor(container, 'OpenAI').classList.contains('motion-reduce:animate-none')).toBe(
			true
		);
	});

	it('stops every moving part when the stream is not live, and keeps the state it last saw', () => {
		const container = draw({ active: [entry('openai')], live: false });
		const edge = edgeFor(container, 'OpenAI');
		const node = nodeFor(container, 'OpenAI');

		expect(edge.classList.contains('animate-flow')).toBe(false);
		expect(edge.classList.contains(DASH)).toBe(false);
		expect(edge.classList.contains('stroke-[var(--color-ok)]')).toBe(true);
		expect(node.querySelector('.animate-ping')).toBeNull();
		expect(node.classList.contains('border-[var(--color-ok)]')).toBe(true);
		expect(within(node).getByText('OpenAI').classList.contains('text-[var(--color-ok)]')).toBe(
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

	it('counts the requests in flight at the gateway, not the nodes that carry them', () => {
		draw({ active: [entry('openai'), entry('openai'), entry('anthropic')] });

		expect(within(gateway()).getByText('3')).toBeTruthy();
	});

	it('shows no count on the gateway when nothing is in flight', () => {
		draw();

		expect(within(gateway()).queryByText('0')).toBeNull();
	});

	it('says in words what the drawing moves, since the drawing is hidden from assistive technology', () => {
		draw();

		expect(
			screen.getByText(/carries a moving dash, its node a soft glow, and the gateway counts/)
		).toBeTruthy();
	});
});
