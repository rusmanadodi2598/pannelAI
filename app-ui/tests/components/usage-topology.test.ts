// Live drawing tests (src/lib/components/UsageTopology.svelte).
//
// The drawing is decorative by construction: it is hidden from assistive technology, and the facts it
// encodes are stated in words by the live row above it. The rows are about the graphic and about
// what the graphic must not carry: which node pulses, what the pulse does when the frame stops naming
// that provider, and which sentences the frame does not state.
//
// The path those nodes form (the combos, the terminals, and the hops between the stages) is next door in
// `usage-topology-path.test.ts`, and the rows about motion being withheld are in
// `usage-topology-motion.test.ts`.

import { cleanup, render, screen, within } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import UsageTopology from '../../src/lib/components/UsageTopology.svelte';
import type { UsageLiveActive } from '$lib/schemas/usage-live';

vi.mock('$app/paths', () => ({ resolve: (path: string) => path }));

const PROVIDERS = [
	{ id: 'openai', name: 'OpenAI' },
	{ id: 'anthropic', name: 'Anthropic' }
];

function entry(providerId: string, model?: string): UsageLiveActive {
	return {
		provider_id: providerId,
		started_at: '2026-09-22T10:00:00Z',
		...(model === undefined ? {} : { model })
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

// `live: true` by default, because these rows are about which node carries a state and a frame is what
// puts one there.
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

/** The node box for a provider, which is the element its label sits in. */
function nodeFor(container: HTMLElement, name: string): HTMLElement {
	return within(container).getByText(name).parentElement as HTMLElement;
}

afterEach(() => {
	cleanup();
});

describe('UsageTopology', () => {
	it('draws one node per configured provider and nothing for an unconfigured one', () => {
		const container = draw({ providers: [{ id: 'openai', name: 'OpenAI' }] });

		expect(within(container).getByText('OpenAI')).toBeTruthy();
		expect(within(container).queryByText('Anthropic')).toBeNull();
	});

	it('explains an empty drawing and links to where a provider is configured', () => {
		const container = draw({ providers: [] });

		expect(screen.getByText('No provider is configured')).toBeTruthy();
		const link = within(container).getByRole('link', { name: 'Open Providers' });
		expect(link.getAttribute('href')).toBe('/providers');
	});

	it('pulses the node that is routing and no other', () => {
		const container = draw({ active: [entry('openai')] });

		expect(nodeFor(container, 'OpenAI').querySelector('.animate-ping')).toBeTruthy();
		expect(nodeFor(container, 'Anthropic').querySelector('.animate-ping')).toBeNull();
	});

	it('drops the pulse as soon as the frame stops naming that provider', () => {
		const container = draw({ active: [entry('openai')] });
		expect(nodeFor(container, 'OpenAI').querySelector('.animate-ping')).toBeTruthy();

		cleanup();
		const idle = draw({ active: [] });

		expect(nodeFor(idle, 'OpenAI').querySelector('.animate-ping')).toBeNull();
	});

	it('lets a reader who asked for less motion turn the pulse off', () => {
		const container = draw({ active: [entry('openai')] });
		const ring = nodeFor(container, 'OpenAI').querySelector('.animate-ping');

		expect(ring?.classList.contains('motion-reduce:hidden')).toBe(true);
	});

	it('carries no sentence of its own, since the live row states the facts', () => {
		// The frame draws, and the words that name what is happening live in the row above
		// (usage-live-row.test.ts). The absence is asserted even with every fact in motion, because a moving
		// fact is exactly when a sentence would most easily reappear here.
		const container = draw({
			active: [entry('openai', 'gpt-4o')],
			last: 'anthropic',
			error: 'openai'
		});
		const figure = container.querySelector('figure');

		// The frame is the positive premise: the silence below only means something while it is drawn.
		expect(figure).toBeTruthy();
		expect(figure?.textContent).not.toMatch(/in flight|Last finished|Last error/);
	});

	it('hides the drawing from assistive technology, since the live row states the facts in words', () => {
		const container = draw({ active: [entry('openai')] });

		expect(container.querySelector('div[aria-hidden="true"]')).toBeTruthy();
	});

	it('sizes a node from the drawing unit, so a narrow box draws a smaller node', () => {
		// Pixel-sized nodes inside a percentage layout intersect at a phone's width, which is why the box
		// carries the unit and every metric is written as its own pixel value times it.
		const container = draw({ providers: [{ id: 'openai', name: 'OpenAI' }] });
		const root = container.querySelector('div[aria-hidden="true"]') as HTMLElement;
		const node = nodeFor(container, 'OpenAI');
		const style = root.getAttribute('style') ?? '';

		expect(root.classList.contains('[container-type:inline-size]')).toBe(true);
		expect(style).toContain('--share: 0.2');
		expect(style).toContain('--u: min(1, calc(var(--share) * tan(atan2(100cqw, 130px))))');
		// The font size is on the box rather than on the drawing: an element is not its own query container, so
		// a font size declared on the drawing resolved against the page and the browser measured 8.4px text in
		// 47px nodes at 390px.
		expect(style).not.toContain('font-size');
		expect(node.classList.contains('[font-size:calc(14px*var(--u))]')).toBe(true);
		expect(node.classList.contains('px-[calc(8px*var(--u))]')).toBe(true);
		expect(node.classList.contains('gap-[calc(8px*var(--u))]')).toBe(true);
		expect(node.querySelector('span')?.classList.contains('size-[calc(8px*var(--u))]')).toBe(true);
		expect(within(node).getByText('OpenAI').classList.contains('max-w-[calc(96px*var(--u))]')).toBe(
			true
		);
	});

	it('draws the routing node label in the status colour and the other states plainly', () => {
		// The colour rule, kept where it always lived: only a routing node's label takes the
		// status colour, and the finished and error nodes keep the default label colour. The row copy of
		// this rule travels with the facts (usage-live-row.test.ts).
		const container = draw({
			providers: [
				{ id: 'openai', name: 'OpenAI' },
				{ id: 'anthropic', name: 'Anthropic' },
				{ id: 'google', name: 'Google' }
			],
			active: [entry('openai')],
			last: 'anthropic',
			error: 'google'
		});

		expect(within(container).getByText('OpenAI').classList.contains('text-[var(--color-ok)]')).toBe(
			true
		);
		expect(
			within(container).getByText('Anthropic').classList.contains('text-[var(--color-ok)]')
		).toBe(false);
		expect(within(container).getByText('Google').classList.contains('text-[var(--color-ok)]')).toBe(
			false
		);
	});
});
