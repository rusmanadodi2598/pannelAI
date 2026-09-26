// The proxy binding card on a provider's detail screen (docs/PORT/009-PORT-PROVIDER-PROXY.md D9;
// docs/SPEC-UI/001-SPEC-UI.md §6.3).
//
// One card for every provider kind is the owner's decision (D1): the reference mounts its per-provider
// card only for a provider with no auth (where there is no connection to carry a pool) and binds a
// keyed provider per connection instead, while this pass binds the provider itself in the settings
// map. So what these cases hold is that both shapes of the screen render it, in the same place, and
// that it sits beside the connections it is read with rather than under the registry's model blocks.

import { cleanup, render, screen, waitFor } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import ProviderDetailPage from '../../src/routes/providers/[provider_id]/+page.svelte';
import { stubModels } from '../support/model-stub';
import { proxyRow } from '../support/proxy-stub';
import { nodeRow } from '../support/provider-node-stub';

vi.mock('$app/paths', () => ({
	resolve: (route: string, params?: Record<string, string>) =>
		params === undefined
			? route
			: route.replace(/\[(\w+)\]/g, (_match, key: string) => params[key] ?? `[${key}]`)
}));

vi.mock('$app/navigation', () => ({ goto: vi.fn(async () => {}) }));

const NODE_ID = 'openai-compatible-01J';

afterEach(() => {
	cleanup();
	vi.unstubAllGlobals();
});

/** The screen's own section headings, in document order, with a closed dialog's excluded. */
function sectionHeadings(): string[] {
	return [...document.querySelectorAll('h2')]
		.filter((element) => element.closest('dialog') === null)
		.map((element) => element.textContent?.replace(/\s+/g, ' ').trim() ?? '');
}

describe('the proxy card on a provider detail screen', () => {
	it('renders on a registry provider, after the connections it belongs beside', async () => {
		stubModels({ proxies: [proxyRow()] });
		render(ProviderDetailPage, { props: { params: { provider_id: 'openai' }, data: {} } });

		expect(await screen.findByLabelText('Proxy pool')).toBeTruthy();
		expect(screen.getByLabelText('Pool strategy')).toBeTruthy();

		const headings = sectionHeadings();
		expect(headings.indexOf('Proxy')).toBeGreaterThan(headings.indexOf('Connections'));
		// The pool rows the card offers are the ones the pool route answered.
		expect(screen.getByRole('option', { name: 'Frankfurt egress' })).toBeTruthy();
	});

	it('renders on a custom node too, which has no registry blocks above it', async () => {
		stubModels({
			providers: ['openai', NODE_ID],
			authType: 'api_key',
			providerNode: nodeRow(),
			proxies: [proxyRow()]
		});
		render(ProviderDetailPage, { props: { params: { provider_id: NODE_ID }, data: {} } });

		expect(await screen.findByLabelText('Proxy pool')).toBeTruthy();

		const headings = sectionHeadings();
		expect(headings.indexOf('Proxy')).toBeGreaterThan(headings.indexOf('Connections'));
		expect(headings.indexOf('Proxy')).toBeLessThan(headings.indexOf('Available Models'));
	});

	it('reports a refused binding on the page rather than silently keeping it', async () => {
		stubModels({ proxies: [proxyRow()], settingsWriteStatus: 400 });
		render(ProviderDetailPage, { props: { params: { provider_id: 'openai' }, data: {} } });
		await screen.findByLabelText('Proxy pool');

		const pool = screen.getByLabelText('Proxy pool') as HTMLSelectElement;
		await waitFor(() => expect(pool.disabled).toBe(false));
		await screen.findByRole('option', { name: 'Frankfurt egress' });
		pool.value = 'prx_01HZZ9K2';
		pool.dispatchEvent(new Event('change', { bubbles: true }));

		expect(await screen.findByText('The gateway refused this value.')).toBeTruthy();
		expect(pool.value).toBe('');
	});
});
