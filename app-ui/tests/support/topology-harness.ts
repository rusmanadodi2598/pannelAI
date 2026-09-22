// The harness the live drawing's component tests share (src/lib/components/UsageTopology.svelte).
//
// Extracted when the beam's rows and the motion gates' rows became two files: both need the same two
// providers, the same props, and the same way of finding a node's edge in the drawing. The mock of
// `$app/paths` stays in each test file, because a mock is hoisted above the imports of the file that
// declares it and the component must not be evaluated before it is registered.

import { render, screen, within } from '@testing-library/svelte';
import UsageTopology from '../../src/lib/components/UsageTopology.svelte';
import type { UsageLiveActive } from '$lib/schemas/usage-live';

export const PROVIDERS = [
	{ id: 'openai', name: 'OpenAI' },
	{ id: 'anthropic', name: 'Anthropic' }
];

/** The active node's glow, which is the reference's `0 0 16px` at a low alpha. */
export const GLOW = 'shadow-[0_0_16px_color-mix(in_srgb,var(--color-ok)_25%,transparent)]';

export function entry(providerId: string): UsageLiveActive {
	return { provider_id: providerId, started_at: '2026-09-22T10:00:00Z' };
}

type Props = {
	providers: { id: string; name: string }[];
	active: UsageLiveActive[];
	last: string;
	error: string;
	live: boolean;
};

export function draw(overrides: Partial<Props> = {}): HTMLElement {
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
export function nodeFor(container: HTMLElement, name: string): HTMLElement {
	return within(container).getByText(name).parentElement as HTMLElement;
}

/**
 * Every line whose far end is a provider's node, matched by the position both are drawn at.
 *
 * Matching on position rather than on order is the point: the drawing's order is the provider list's
 * order, and a row about "only this provider's line moves" must not pass because two lists happen to
 * agree.
 */
export function linesAt(container: HTMLElement, name: string): SVGLineElement[] {
	const box = nodeFor(container, name);
	const x = Number.parseFloat(box.style.left);
	const y = Number.parseFloat(box.style.top);
	return [...container.querySelectorAll('line')].filter(
		(line) =>
			Number.parseFloat(line.getAttribute('x2') ?? '') === x &&
			Number.parseFloat(line.getAttribute('y2') ?? '') === y
	);
}

/** The beam's own lines at a node, keyed by the part they draw. */
export function beamAt(container: HTMLElement, name: string, part: string): SVGLineElement[] {
	return linesAt(container, name).filter((line) => line.getAttribute('data-beam') === part);
}

/** The one line an edge that is not carrying a beam is drawn as. */
export function edgeFor(container: HTMLElement, name: string): SVGLineElement {
	const base = linesAt(container, name).filter((line) => line.getAttribute('data-beam') === null);
	if (base.length !== 1) throw new Error(`${name} has ${base.length} plain edges, expected one`);
	return base[0];
}

export function gateway(): HTMLElement {
	return screen.getByText('Gateway').parentElement as HTMLElement;
}
