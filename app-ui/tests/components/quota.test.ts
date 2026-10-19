// Quota Tracker, the screen's own reads (docs/SPEC-UI/001-SPEC-UI.md §6.6).
//
// What an operator sees when the data lands: one card per provider (docs/PORT/005-PORT-QUOTA-CARDS.md D1),
// the counted windows as one summary line per connection rather than a row set, the endpoint label resolved
// or the identifier admitted, and the two source sentences that keep the gateway's numbers and the
// provider's apart. Folding, the paged reads and the polling loop are each their own file: this one is what
// the card says.

import { cleanup, fireEvent, render, screen } from '@testing-library/svelte';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import QuotaPage from '../../src/routes/quota/+page.svelte';
import { quotaCountedLine, quotaEndpointRow, quotaWindowRow } from '../support/quota-fixtures';
import { stubQuota } from '../support/quota-stub';
import { visit } from '../support/page.svelte';

vi.mock('$app/paths', () => ({ resolve: (path: string) => path }));

describe('QuotaPage', () => {
	beforeEach(() => {
		visit('/quota');
	});

	afterEach(() => {
		cleanup();
		vi.unstubAllGlobals();
	});

	it('renders one card per provider, its endpoints as rows inside', async () => {
		stubQuota({
			windows: [
				quotaWindowRow(),
				quotaWindowRow({ window: 'daily', endpoint_id: 'ep_1b' }),
				quotaWindowRow({ provider_id: 'zeta', endpoint_id: 'ep_z' })
			],
			endpoints: [
				quotaEndpointRow(),
				quotaEndpointRow({ id: 'ep_1b', label: 'Anthropic secondary' }),
				quotaEndpointRow({ id: 'ep_z', label: 'Zeta primary' })
			]
		});
		render(QuotaPage);

		// One card heading per provider, first seen first; the endpoints inside are row headings, not cards.
		expect(await screen.findByRole('heading', { name: 'anthropic' })).toBeTruthy();
		expect(screen.getAllByRole('heading', { name: 'anthropic' }).length).toBe(1);
		expect(screen.getByRole('heading', { name: 'zeta' })).toBeTruthy();
		expect(screen.getByRole('heading', { name: 'Anthropic primary' })).toBeTruthy();
		expect(screen.getByRole('heading', { name: 'Anthropic secondary' })).toBeTruthy();
		// The header counts tell a folded card's story without opening it.
		expect(screen.getByText('2 endpoints, 2 windows')).toBeTruthy();
		expect(screen.getByText('1 endpoint, 1 window')).toBeTruthy();
	});

	it('summarises the counted window by kind, spend, share, and when it reopens', async () => {
		stubQuota({ windows: [quotaWindowRow()] });
		render(QuotaPage);

		expect(await screen.findByRole('heading', { name: 'anthropic' })).toBeTruthy();
		// One line for this gateway's own count, carrying what the old row set carried: which window,
		// how much of its ceiling went, the percentage the rows above use, and the countdown.
		const line = screen.getByText(quotaCountedLine('monthly'));
		expect(line.textContent).toContain('1 window');
		expect(line.textContent).toContain('120,000 / 200,000');
		expect(line.textContent).toContain('(60%)');
		expect(line.textContent).toMatch(/resets in (2h|1h 59m)/);
	});

	it('keeps the provider order first seen', async () => {
		stubQuota({
			windows: [
				quotaWindowRow(),
				quotaWindowRow({ provider_id: 'zeta', endpoint_id: 'ep_z', window: 'daily' }),
				quotaWindowRow({ window: 'daily' })
			]
		});
		render(QuotaPage);

		const headings = await screen.findAllByRole('heading', { name: /anthropic|zeta/ });
		expect(headings.map((heading) => heading.textContent)).toEqual(['anthropic', 'zeta']);
	});

	it('carries the icon map glyphs on the refresh and pause controls, never an emoticon', async () => {
		stubQuota({ windows: [quotaWindowRow()] });
		render(QuotaPage);
		await screen.findByRole('heading', { name: 'anthropic' });

		// R-04: the glyphs are the map's marks, rendered as svg markup; an emoji in the label would be
		// the generated-page tell the icon map exists to prevent.
		const refresh = screen.getByRole('button', { name: 'Refresh now' });
		expect(refresh.querySelector('svg')).toBeTruthy();
		const pause = screen.getByRole('button', { name: 'Pause refresh' });
		expect(pause.querySelector('svg')).toBeTruthy();

		await fireEvent.click(pause);
		const resume = screen.getByRole('button', { name: 'Resume refresh' });
		expect(resume.querySelector('svg')).toBeTruthy();
	});

	it('names a window the gateway recorded without a provider, instead of refusing the list', async () => {
		stubQuota({ windows: [quotaWindowRow({ provider_id: '' })] });
		render(QuotaPage);

		// The free lane's virtual endpoint carries no provider. One row like that used to reject the
		// whole read; now the card renders with the sentence that says the counts are local.
		expect(await screen.findByRole('heading', { name: 'No provider' })).toBeTruthy();
		// The lane's own sentence and its counted line: with no provider behind it there is nothing else
		// on the card, which is why the summary keeps the window kind and the countdown.
		expect(screen.getByText(/Counted locally by this gateway/)).toBeTruthy();
		expect(screen.getByText(quotaCountedLine('monthly'))).toBeTruthy();
	});

	it('resolves an endpoint label, and names the identifier when it has no label', async () => {
		stubQuota({
			windows: [quotaWindowRow(), quotaWindowRow({ endpoint_id: 'ep_beyond_the_page' })]
		});
		render(QuotaPage);

		expect(await screen.findByRole('heading', { name: 'Anthropic primary' })).toBeTruthy();
		expect(screen.getByRole('heading', { name: 'ep_beyond_the_page' })).toBeTruthy();
	});

	it('says a window has no ceiling rather than reporting 0%', async () => {
		stubQuota({ windows: [quotaWindowRow({ limit: undefined })] });
		render(QuotaPage);

		// A ceiling the gateway never recorded is stated as absent, not as a spent one and not as 0%.
		expect(await screen.findByText(/no ceiling/)).toBeTruthy();
		expect(screen.queryByText('0%')).toBeNull();
	});

	it('explains both source values under the cards', async () => {
		stubQuota({ windows: [quotaWindowRow()] });
		render(QuotaPage);
		await screen.findByText(/Computed: counted by this gateway/);

		// §6.6 calls the badge functional, so what it means cannot live in a hover-only tooltip (§8.7.5).
		expect(screen.getByText(/Computed: counted by this gateway/)).toBeTruthy();
		expect(screen.getByText(/Reported: published by the provider/)).toBeTruthy();
	});

	it('states the interval it refreshes on and where it stops', async () => {
		stubQuota({ windows: [quotaWindowRow()] });
		render(QuotaPage);
		await screen.findByText(/Auto-refresh every 30 seconds/);

		expect(screen.getByText(/Auto-refresh every 30 seconds/)).toBeTruthy();
	});

	it('shows the empty state §6.6 specifies', async () => {
		stubQuota({ windows: [] });
		render(QuotaPage);

		expect(await screen.findByText('No quota windows yet')).toBeTruthy();
		expect(screen.getByText(/No traffic yet\. Send one request/)).toBeTruthy();
	});

	it('keeps the cards and says so when the endpoint labels cannot be read', async () => {
		stubQuota({ windows: [quotaWindowRow()], endpoints: null });

		render(QuotaPage);

		expect(await screen.findByText(/Endpoint labels could not be read/)).toBeTruthy();
		// The card is still rendered, its endpoint named by the identifier.
		expect(screen.getByRole('heading', { name: 'ep_1' })).toBeTruthy();
	});
});
