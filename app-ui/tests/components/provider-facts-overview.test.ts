// The Overview tab of a registry provider (docs/SPEC-UI/001-SPEC-UI.md §6.3, "Tabs").
//
// Measured before the change: a flat list of eleven label/value pairs, with a status-free "No endpoint
// configured" line that read like a category name. The contract here is what an operator reads from it:
// the facts are grouped into three cards by the question each answers, the first thing read is a status
// line whose sentence matches the reason, and the words carry the meaning without the colour.

import { cleanup, render, screen, within } from '@testing-library/svelte';
import { afterEach, describe, expect, it } from 'vitest';
import ProviderFacts from '../../src/lib/components/ProviderFacts.svelte';
import { expectIconOnly } from '../support/icon-only';
import type { ProviderDetail } from '../../src/lib/schemas/provider';

afterEach(() => {
	cleanup();
});

function detail(overrides: Partial<ProviderDetail> = {}): ProviderDetail {
	return {
		id: 'opencode',
		name: 'OpenCode',
		category: 'free',
		auth_type: 'none',
		auth_modes: [],
		auth_hint: null,
		has_oauth: false,
		no_auth: true,
		routability: 'native',
		endpoint_count: 0,
		status_summary: { total: 0, active: 0, disabled: 0, error: 0, rate_limited: 0 },
		base_url: 'https://opencode.ai',
		format: 'openai',
		url_suffix: '',
		validate_url: '',
		timeout_ms: 0,
		model_count: 4,
		chat_model_count: 3,
		media: [],
		deprecated: false,
		...overrides
	} as ProviderDetail;
}

describe('the Overview facts on a registry provider', () => {
	it('groups the facts into three cards, each with its own heading', () => {
		render(ProviderFacts, { props: { provider: detail() } });

		for (const title of ['Transport', 'Access', 'Models and endpoints']) {
			expect(screen.getByRole('heading', { name: title })).toBeTruthy();
		}
		// Each card is a labelled region, so a screen reader announces the group, not one long list.
		expect(screen.getByRole('region', { name: 'Transport' })).toBeTruthy();
	});

	it('puts the transport facts in the Transport card and the access facts in the Access card', () => {
		render(ProviderFacts, { props: { provider: detail() } });

		const transport = screen.getByRole('region', { name: 'Transport' });
		expect(within(transport).getByText('Base URL')).toBeTruthy();
		expect(within(transport).getByText('Format')).toBeTruthy();
		expect(within(transport).getByText('Timeout')).toBeTruthy();

		const access = screen.getByRole('region', { name: 'Access' });
		expect(within(access).getByText('Category')).toBeTruthy();
		expect(within(access).getByText('Auth')).toBeTruthy();
	});

	it('states a zero timeout as no timeout, not as a bare 0 ms', () => {
		render(ProviderFacts, { props: { provider: detail({ timeout_ms: 0 }) } });
		expect(screen.getByText('No timeout')).toBeTruthy();
		expect(screen.queryByText('0 ms')).toBeNull();
	});

	it('shows a configured timeout in milliseconds', () => {
		render(ProviderFacts, { props: { provider: detail({ timeout_ms: 30000 }) } });
		expect(screen.getByText('30000 ms')).toBeTruthy();
	});

	it('offers an icon-only copy control on the base URL', () => {
		render(ProviderFacts, { props: { provider: detail() } });

		// The control keeps the action as its accessible name and its tooltip; the glyph is the whole visual,
		// which is what keeps the row compact beside the value it copies.
		const copy = screen.getByRole('button', { name: 'Copy base URL' });
		expectIconOnly(copy, 'Copy base URL');
		// A standalone control in a card header is the dense 36px size, the same as the dialogs' copy controls.
		expect(copy.className).toContain('size-9');
	});

	it('places the base URL copy in the Transport card heading, not in the value', () => {
		render(ProviderFacts, { props: { provider: detail() } });
		const transport = screen.getByRole('region', { name: 'Transport' });
		const copy = within(transport).getByRole('button', { name: 'Copy base URL' });

		// The copy is an action on the card, so it sits in the heading row beside the card title, and the
		// Base URL value stays plain text the copy reads from.
		const heading = within(transport).getByRole('heading', { name: 'Transport' });
		expect(heading.parentElement?.contains(copy)).toBe(true);
		const baseValue = within(transport).getByText('Base URL').nextElementSibling as HTMLElement;
		expect(baseValue.contains(copy)).toBe(false);
	});

	it('leaves out the copy control when there is no base URL to copy', () => {
		render(ProviderFacts, { props: { provider: detail({ base_url: '' }) } });
		expect(screen.queryByRole('button', { name: 'Copy base URL' })).toBeNull();

		// The Base URL row states the value is missing rather than leaving the cell blank.
		const transport = screen.getByRole('region', { name: 'Transport' });
		const baseRow = within(transport).getByText('Base URL').closest('div') as HTMLElement;
		expect(within(baseRow).getByText('Not set')).toBeTruthy();
	});

	it.each([
		{
			name: 'a native provider with no endpoint says it has none yet',
			overrides: {},
			text: 'No endpoint yet'
		},
		{
			name: 'a connector says it is not routable, whatever its endpoints',
			overrides: { routability: 'connector' },
			text: 'Not routable'
		},
		{
			name: 'a failing endpoint says so',
			overrides: {
				endpoint_count: 1,
				status_summary: { total: 1, active: 0, disabled: 0, error: 1, rate_limited: 0 }
			},
			text: 'Endpoints failing'
		},
		{
			name: 'a provider with an active endpoint routes',
			overrides: {
				endpoint_count: 1,
				status_summary: { total: 1, active: 1, disabled: 0, error: 0, rate_limited: 0 }
			},
			text: 'Routing'
		}
	])('the status line reads right when $name', ({ overrides, text }) => {
		render(ProviderFacts, { props: { provider: detail(overrides as Partial<ProviderDetail>) } });
		// The verdict is the provider-level status; a deprecation notice is a separate status block.
		const statuses = screen.getAllByRole('status');
		const verdict = statuses.find((s) => /routability/.test(s.textContent ?? '')) as HTMLElement;
		expect(verdict, 'the status line is not on screen').toBeTruthy();
		expect(within(verdict).getByText(text)).toBeTruthy();
	});

	it('marks the endpoint count as a warning when the provider has none', () => {
		render(ProviderFacts, { props: { provider: detail() } });
		const endpoints = screen.getByText('No endpoint configured');
		expect(endpoints.className).toContain('text-[var(--color-warn)]');
	});
});
