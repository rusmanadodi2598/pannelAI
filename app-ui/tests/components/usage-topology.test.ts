// Live drawing tests (src/lib/components/UsageTopology.svelte, draft 012 F3).
//
// The drawing is decorative by construction: it is hidden from assistive technology and the sentences
// beneath it carry every fact it encodes. So the rows come in pairs, one for the graphic and one for the
// text, and the interesting ones are the states: which node pulses, and what the pulse does when the frame
// stops naming that provider.

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

	it('states the configured providers, so the drawing is not the only record of them', () => {
		draw();

		expect(screen.getByText('2 providers are configured: OpenAI, Anthropic.')).toBeTruthy();
	});

	it('names what is in flight, with the model the frame reported', () => {
		draw({ active: [entry('openai', 'gpt-4o'), entry('anthropic')] });

		expect(screen.getByText(/2 in flight: OpenAI \(gpt-4o\), Anthropic\./)).toBeTruthy();
	});

	it('says nothing is in flight rather than leaving the line out', () => {
		draw();

		expect(screen.getByText(/No request is in flight\./)).toBeTruthy();
	});

	it('names the provider the last request finished on, and the one that errored', () => {
		draw({ last: 'anthropic', error: 'openai' });

		expect(screen.getByText(/The last request to finish went to Anthropic\./)).toBeTruthy();
		expect(screen.getByText(/The gateway last reported an error on OpenAI\./)).toBeTruthy();
	});

	it('says nothing has finished rather than claiming a last request', () => {
		draw();

		expect(screen.getByText(/No request has finished since this screen opened\./)).toBeTruthy();
	});

	it('names a provider the registry does not carry by its id', () => {
		draw({ last: 'gone-provider' });

		expect(screen.getByText(/The last request to finish went to gone-provider\./)).toBeTruthy();
	});

	it('hides the drawing from assistive technology, since the sentences carry the same facts', () => {
		const container = draw({ active: [entry('openai')] });

		expect(container.querySelector('div[aria-hidden="true"]')).toBeTruthy();
	});

	it('explains the colours in words', () => {
		draw();

		expect(
			screen.getByText(/Green is routing now, amber finished last, red is where/)
		).toBeTruthy();
	});
});
