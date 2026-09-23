// Live drawing tests (src/lib/components/UsageTopology.svelte, draft 012 F3).
//
// The drawing is decorative by construction: it is hidden from assistive technology, and the facts it
// encodes that are not on the screen in words are stated in words above it. So the rows come in pairs,
// one for the graphic and one for the text, and the interesting ones are the states: which node pulses,
// what the pulse does when the frame stops naming that provider, and which facts the words state and which
// they leave to the drawing (owner's correction, 2026-09-23, draft 022 F2 and draft 023 F1).

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
	active: UsageLiveActive[];
	last: string;
	error: string;
	live: boolean;
};

// `live: true` by default, because these rows are about which node carries a state and a frame is what
// puts one there. The rows about motion being withheld belong to `usage-topology-motion.test.ts`.
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

	it('states no fact in words while the drawing is idle', () => {
		// The owner's correction of 2026-09-23: the provider list repeats the node labels, and "no request
		// is in flight" / "no request has finished" state only an absence, so an idle screen carries no
		// sentence that never changes. The line keeps its slot, which is why the absence is asserted on the
		// facts rather than on an empty paragraph.
		draw();

		expect(screen.queryByText(/in flight:/)).toBeNull();
		expect(screen.queryByText(/Last finished:/)).toBeNull();
		expect(screen.queryByText(/Last error:/)).toBeNull();
	});

	it('names what is in flight, with the model the frame reported', () => {
		draw({ active: [entry('openai', 'gpt-4o'), entry('anthropic')] });

		expect(screen.getByText('2 in flight:')).toBeTruthy();
		expect(screen.getByText('OpenAI (gpt-4o), Anthropic.')).toBeTruthy();
	});

	it('names the provider the last request finished on, and the one that errored', () => {
		draw({ last: 'anthropic', error: 'openai' });

		expect(screen.getByText('Last finished:')).toBeTruthy();
		expect(screen.getByText('Anthropic.')).toBeTruthy();
		expect(screen.getByText('Last error:')).toBeTruthy();
		expect(screen.getByText('OpenAI.')).toBeTruthy();
	});

	it('names a provider the registry does not carry by its id', () => {
		draw({ last: 'gone-provider' });

		expect(screen.getByText('gone-provider.')).toBeTruthy();
	});

	it('states the facts above the drawing, where the explanation used to sit', () => {
		// Draft 023 F1: the paragraph moved out, the facts moved up, and the drawing is the thing below.
		const container = draw({ active: [entry('openai')] });
		const line = within(container).getByText('1 in flight:');
		const drawing = container.querySelector('div[aria-hidden="true"]') as HTMLElement;

		expect(line.compareDocumentPosition(drawing) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
	});

	it('hides the drawing from assistive technology, since the live facts are also stated in words', () => {
		const container = draw({ active: [entry('openai')] });

		expect(container.querySelector('div[aria-hidden="true"]')).toBeTruthy();
	});

	it('sizes a node from the drawing unit, so a narrow box draws a smaller node', () => {
		// Draft 016 F8: the node boxes were pixel-sized inside a percentage layout, so at 390px two of them
		// intersected by 47px. The box now carries the unit and every metric is written as its own pixel
		// value times it (draft 018 F1).
		const container = draw({ providers: [{ id: 'openai', name: 'OpenAI' }] });
		const root = container.querySelector('div[aria-hidden="true"]') as HTMLElement;
		const node = nodeFor(container, 'OpenAI');
		const style = root.getAttribute('style') ?? '';

		expect(root.classList.contains('[container-type:inline-size]')).toBe(true);
		expect(style).toContain('--share: 0.2');
		expect(style).toContain('--u: min(1, calc(var(--share) * tan(atan2(100cqw, 130px))))');
		// The font size is on the box rather than on the drawing: an element is not its own query container, so
		// a font size declared on the drawing resolved against the page and the browser measured 8.4px text in
		// 47px nodes at 390px (draft 018 F1).
		expect(style).not.toContain('font-size');
		expect(node.classList.contains('[font-size:calc(14px*var(--u))]')).toBe(true);
		expect(node.classList.contains('px-[calc(8px*var(--u))]')).toBe(true);
		expect(node.classList.contains('gap-[calc(8px*var(--u))]')).toBe(true);
		expect(node.querySelector('span')?.classList.contains('size-[calc(8px*var(--u))]')).toBe(true);
		expect(within(node).getByText('OpenAI').classList.contains('max-w-[calc(96px*var(--u))]')).toBe(
			true
		);
	});

	it('gives the routing name the status colour, and no colour to the other two facts', () => {
		// Draft 023 F1: the paragraph that defined the colours is gone, so the line carries the drawing's
		// own rule instead. In the drawing only a routing node's label takes the status colour; the
		// finished and error nodes keep the default label colour, and so do their facts here.
		draw({
			providers: [
				{ id: 'openai', name: 'OpenAI' },
				{ id: 'anthropic', name: 'Anthropic' },
				{ id: 'google', name: 'Google' }
			],
			active: [entry('openai')],
			last: 'anthropic',
			error: 'google'
		});

		expect(screen.getByText('OpenAI.').classList.contains('text-[var(--color-ok)]')).toBe(true);
		expect(screen.getByText('Anthropic.').classList.contains('text-[var(--color-ok)]')).toBe(false);
		expect(screen.getByText('Google.').classList.contains('text-[var(--color-ok)]')).toBe(false);
	});
});
