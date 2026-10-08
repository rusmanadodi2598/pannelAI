// Combo band tests for the live panel (src/lib/components/UsageLivePanel.svelte).
//
// Split from `usage-live-drawing.test.ts` on the same seam that file was split on: the rows there are about
// the reads that existed when the drawing only knew providers, and these are about the third read
// (the combo list) and the band it puts above the gateway. The two subjects fail for different reasons, so
// they belong in different files: one is "the registry landed", the other is "the drawing can name the
// combo a request entered through".

import { cleanup, render, screen } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import UsageLivePanel from '../../src/lib/components/UsageLivePanel.svelte';
import { frameText } from '../support/live-stream';
import { comboRow, frame, startedNow, stubPanel } from '../support/live-panel-stub';

vi.mock('$app/paths', () => ({ resolve: (path: string) => path }));

afterEach(() => {
	cleanup();
	vi.unstubAllGlobals();
});

describe('UsageLivePanel combo band', () => {
	it('draws a node per combo, on the band above the gateway', async () => {
		stubPanel({ combos: [comboRow('pro-tier'), comboRow('fast')] });
		const container = render(UsageLivePanel).container;

		expect(await screen.findByText('pro-tier')).toBeTruthy();
		expect(screen.getByText('fast')).toBeTruthy();

		const combo = (screen.getByText('pro-tier').parentElement as HTMLElement).style.top;
		const upstream = (screen.getByText('OpenAI').parentElement as HTMLElement).style.top;

		expect(Number.parseFloat(combo)).toBeLessThan(50);
		expect(Number.parseFloat(upstream)).toBeGreaterThan(50);
		// The band is drawn from the combo read alone, so a combo nothing is routing still occupies its
		// place. That is the drawing's idle state, not a claim about now.
		expect(container.querySelector('.animate-ping')).toBeNull();
	});

	it('names the combo a request entered through, in the row as well as on the band', async () => {
		const stub = stubPanel();
		render(UsageLivePanel);
		await screen.findByText('Connecting');

		stub.streams[0].send(
			frameText(
				frame({
					active: [
						{
							provider_id: 'openai',
							model: 'gpt-4o',
							combo: 'pro-tier',
							started_at: startedNow()
						}
					]
				})
			)
		);

		// The drawing is aria-hidden, so the row is the only place this fact exists for a reader who cannot
		// see which band lit up (SPEC-UI §6.5).
		expect(await screen.findByText('pro-tier → OpenAI (gpt-4o)')).toBeTruthy();
		expect(
			(screen.getByText('pro-tier').parentElement as HTMLElement).classList.contains(
				'border-[var(--color-ok)]'
			)
		).toBe(true);
	});

	it('states which combos the drawing leaves out', async () => {
		stubPanel({ combos: [comboRow('pro-tier')], combosTotal: 12 });
		render(UsageLivePanel);

		expect(await screen.findByText(/Shows the first 1 of 12 combos\./)).toBeTruthy();
	});

	it('states that the combo list could not be read, and still draws the upstreams it can', async () => {
		stubPanel({ combosStatus: 500, combosMessage: 'the combo list is unreachable' });
		const container = render(UsageLivePanel);

		expect(
			await screen.findByText(/Combos could not be read\. The drawing has no combo band\./)
		).toBeTruthy();
		// The provider band is a separate read, and one failing read must not blank the drawing the other
		// one could fill.
		expect(await screen.findByText('OpenAI')).toBeTruthy();
		expect(container.container.querySelectorAll('[data-edge="combo-gateway"]')).toHaveLength(0);
	});
});
