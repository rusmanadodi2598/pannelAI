// Upstream endpoints tab state tests (docs/SPEC-UI/001-SPEC-UI.md §6.2 tab 2, §8.3, §8.4.2).
//
// §8.3 requires the cause of an empty view to be named, and this tab has three of them: a filter that
// matched nothing, a registry with no endpoints at all, and a page past the end of the list. They call for
// different actions from the operator, so each is asserted against the others rather than on its own.
//
// The option read is the tab's second read and it is deliberately unfiltered, so one case here proves that
// it stays unfiltered while the table is filtered, and another proves the tab says so when it fails instead
// of presenting a short list as the whole registry.

import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/svelte';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { SvelteURLSearchParams } from 'svelte/reactivity';
import UpstreamEndpointsTab from '../../src/lib/components/UpstreamEndpointsTab.svelte';
import { endpointRow, stubEndpoints, type EndpointStub } from '../support/endpoint-stub';
import { pageState, queryOf, visit } from '../support/page.svelte';

vi.mock('$app/state', async () => {
	const { pageState: page } = await import('../support/page.svelte');
	return { page };
});

vi.mock('$app/paths', () => ({ resolve: (path: string) => path }));

vi.mock('$app/navigation', async () => {
	const { pageState: page, queryOf: query } = await import('../support/page.svelte');
	return {
		goto: (url: string) => {
			page.url = {
				pathname: '/endpoint-keys',
				searchParams: new SvelteURLSearchParams(query(url))
			};
			return Promise.resolve();
		}
	};
});

function twoRows(): Record<string, unknown>[] {
	return [
		endpointRow({ id: 'ep_1', provider_id: 'openai', provider_name: 'OpenAI', label: 'Primary' }),
		endpointRow({
			id: 'ep_2',
			provider_id: 'anthropic',
			provider_name: 'Anthropic',
			label: 'Secondary',
			priority: 2
		})
	];
}

/** The query of the unfiltered provider-option read, which asks for the API's per_page cap. */
function optionQuery(stub: EndpointStub): URLSearchParams {
	const reads = stub.requested.filter((url) => {
		const parsed = new URL(url, 'http://panel.test');
		return parsed.pathname.endsWith('/endpoints') && parsed.searchParams.get('per_page') === '100';
	});
	return new URLSearchParams(queryOf(reads[reads.length - 1] ?? ''));
}

async function renderTab(): Promise<void> {
	render(UpstreamEndpointsTab);
	await screen.findByRole('table');
}

describe('UpstreamEndpointsTab states', () => {
	beforeEach(() => {
		visit('/endpoint-keys');
	});

	afterEach(() => {
		cleanup();
		vi.unstubAllGlobals();
	});

	it('names a filter that matched nothing, and offers to clear it', async () => {
		visit('/endpoint-keys', 'provider_id=ghost');
		stubEndpoints({ rows: twoRows() });
		render(UpstreamEndpointsTab);

		expect(await screen.findByText('No endpoint matches these filters')).toBeTruthy();
		expect(screen.queryByText('No upstream endpoints yet')).toBeNull();
		expect(screen.queryByText('Nothing on this page')).toBeNull();
	});

	it('names an empty registry, which is a different problem from an empty filter', async () => {
		stubEndpoints({ rows: [] });
		render(UpstreamEndpointsTab);

		expect(await screen.findByText('No upstream endpoints yet')).toBeTruthy();
		expect(screen.queryByText('No endpoint matches these filters')).toBeNull();
	});

	it('names a page past the end of the list, and offers the first page', async () => {
		visit('/endpoint-keys', 'page=9');
		stubEndpoints({ rows: twoRows() });
		render(UpstreamEndpointsTab);

		expect(await screen.findByText('Nothing on this page')).toBeTruthy();
		expect(screen.queryByText('No upstream endpoints yet')).toBeNull();

		await fireEvent.click(screen.getByRole('button', { name: 'Go to the first page' }));

		await waitFor(() => {
			expect(pageState.url.searchParams.get('page')).toBeNull();
		});
		expect(await screen.findByRole('table')).toBeTruthy();
	});

	it('offers the provider the URL names even when no read returned it', async () => {
		visit('/endpoint-keys', 'provider_id=ghost');
		stubEndpoints({ rows: twoRows() });
		render(UpstreamEndpointsTab);

		await screen.findByText('No endpoint matches these filters');

		const select = screen.getByLabelText('Provider') as HTMLSelectElement;
		// A select whose value has no matching option renders blank, which would hide the filter in force.
		expect(select.value).toBe('ghost');
		expect(within(select).getByRole('option', { name: 'ghost' })).toBeTruthy();
	});

	it('keeps the option read unfiltered while the table is filtered', async () => {
		visit('/endpoint-keys', 'provider_id=anthropic');
		const stub = stubEndpoints({ rows: twoRows() });
		await renderTab();

		const options = optionQuery(stub);

		expect(options.get('provider_id')).toBeNull();
		expect(options.get('status')).toBeNull();
		// Both providers are offered, so the operator can switch without clearing the filter first.
		const select = screen.getByLabelText('Provider') as HTMLSelectElement;
		expect(within(select).getByRole('option', { name: 'OpenAI' })).toBeTruthy();
		expect(within(select).getByRole('option', { name: 'Anthropic' })).toBeTruthy();
	});

	it('says why the provider list is short when its own read fails', async () => {
		const stub = stubEndpoints({ rows: twoRows(), optionsStatus: 500 });
		await renderTab();

		expect(await screen.findByText(/provider list could not be read/)).toBeTruthy();
		// The filter still offers what the result carried rather than presenting an empty select.
		const select = screen.getByLabelText('Provider') as HTMLSelectElement;
		expect(within(select).getByRole('option', { name: 'OpenAI' })).toBeTruthy();
		expect(stub.requested.length).toBeGreaterThan(0);
	});

	it('reports what it corrected in the address', async () => {
		visit('/endpoint-keys', 'status=broken');
		stubEndpoints({ rows: twoRows() });
		await renderTab();

		expect(await screen.findByText(/status "broken" is not one the panel offers/)).toBeTruthy();
	});

	it('reports a failed read and re-reads when the operator retries', async () => {
		const stub = stubEndpoints({ rows: twoRows(), listStatus: 500 });
		render(UpstreamEndpointsTab);

		expect(await screen.findByText('Upstream endpoints could not be loaded')).toBeTruthy();

		stub.listStatus = 200;
		const before = stub.requested.length;
		await fireEvent.click(screen.getByRole('button', { name: 'Try again' }));

		await waitFor(() => {
			expect(stub.requested.length).toBeGreaterThan(before);
		});
		expect(await screen.findByRole('table')).toBeTruthy();
	});
});
