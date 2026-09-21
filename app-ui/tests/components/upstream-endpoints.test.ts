// Upstream endpoints tab tests (docs/SPEC-UI/001-SPEC-UI.md §6.2 tab 2, §8.4.1, §8.4.2).
//
// The stub narrows its own rows by the query it receives, so "the filter narrowed the table" is proved by
// the request the panel sent rather than by a second filter inside the test. That is what F5 is about: the
// tab used to compute the filter locally and then render the whole page anyway, so a local filter and a
// server filter were indistinguishable from the screen.

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

function threeRows(): Record<string, unknown>[] {
	return [
		endpointRow({ id: 'ep_1', provider_id: 'openai', provider_name: 'OpenAI', label: 'Primary' }),
		endpointRow({
			id: 'ep_2',
			provider_id: 'anthropic',
			provider_name: 'Anthropic',
			label: 'Secondary',
			status: 'disabled',
			priority: 2
		}),
		endpointRow({
			id: 'ep_3',
			provider_id: 'anthropic',
			provider_name: 'Anthropic',
			label: 'Tertiary',
			priority: 3
		})
	];
}

/**
 * The query of the page read, which is the one that carries the filters.
 *
 * The tab makes a second read for the provider options, and that one asks for the API's per_page cap, so
 * the two are told apart by that parameter rather than by call order.
 */
function pageQuery(stub: EndpointStub): URLSearchParams {
	const reads = stub.requested.filter((url) => {
		const parsed = new URL(url, 'http://panel.test');
		return parsed.pathname.endsWith('/endpoints') && parsed.searchParams.get('per_page') !== '100';
	});
	return new URLSearchParams(queryOf(reads[reads.length - 1] ?? ''));
}

function table(): HTMLElement {
	return screen.getByRole('table');
}

async function renderTab(): Promise<void> {
	render(UpstreamEndpointsTab);
	await screen.findByRole('table');
}

/**
 * Renders and waits for the first page read, for the cases whose page holds no rows.
 *
 * A shared address can name a page the filtered set no longer has, and the tab answers that with the
 * "past the end" state rather than a table, so waiting for a table would assert the wrong thing.
 */
async function renderLoaded(stub: EndpointStub): Promise<void> {
	render(UpstreamEndpointsTab);
	await waitFor(() => {
		expect(pageQuery(stub).get('page')).not.toBeNull();
	});
}

describe('UpstreamEndpointsTab filters', () => {
	beforeEach(() => {
		visit('/endpoint-keys');
	});

	afterEach(() => {
		cleanup();
		vi.unstubAllGlobals();
	});

	it('asks for the first page with no filter when the address is bare', async () => {
		const stub = stubEndpoints({ rows: threeRows() });
		await renderTab();

		const query = pageQuery(stub);

		expect(query.get('provider_id')).toBeNull();
		expect(query.get('status')).toBeNull();
		expect(query.get('page')).toBe('1');
		expect(query.get('per_page')).toBe('25');
	});

	it('sends every filter the URL carries', async () => {
		visit('/endpoint-keys', 'provider_id=anthropic&status=disabled&page=2');
		const stub = stubEndpoints({ rows: threeRows() });
		await renderLoaded(stub);

		const query = pageQuery(stub);

		expect(query.get('provider_id')).toBe('anthropic');
		expect(query.get('status')).toBe('disabled');
		expect(query.get('page')).toBe('2');
	});

	it('renders the rows the server returned for the filter, not every row it holds', async () => {
		visit('/endpoint-keys', 'provider_id=anthropic');
		stubEndpoints({ rows: threeRows() });
		await renderTab();

		const cells = within(table());

		expect(cells.getByText('Secondary')).toBeTruthy();
		expect(cells.getByText('Tertiary')).toBeTruthy();
		expect(cells.queryByText('Primary')).toBeNull();
	});

	it('narrows by status as well, which is the second filter §6.2 asks for', async () => {
		visit('/endpoint-keys', 'status=disabled');
		stubEndpoints({ rows: threeRows() });
		await renderTab();

		const cells = within(table());

		expect(cells.getByText('Secondary')).toBeTruthy();
		expect(cells.queryByText('Primary')).toBeNull();
		expect(cells.queryByText('Tertiary')).toBeNull();
	});

	it('writes the provider filter to the URL and asks the server with it', async () => {
		const stub = stubEndpoints({ rows: threeRows() });
		await renderTab();

		await fireEvent.change(screen.getByLabelText('Provider'), { target: { value: 'anthropic' } });

		await waitFor(() => {
			expect(pageState.url.searchParams.get('provider_id')).toBe('anthropic');
		});
		await waitFor(() => {
			expect(pageQuery(stub).get('provider_id')).toBe('anthropic');
		});
		// The excluded row is gone because the server did not return it, not because the table hid it.
		expect(within(table()).queryByText('Primary')).toBeNull();
	});

	it('writes the status filter and returns to the first page', async () => {
		visit('/endpoint-keys', 'page=2');
		const stub = stubEndpoints({ rows: threeRows() });
		await renderLoaded(stub);

		await fireEvent.change(screen.getByLabelText('Status'), { target: { value: 'active' } });

		await waitFor(() => {
			expect(pageState.url.searchParams.get('status')).toBe('active');
		});
		expect(pageState.url.searchParams.get('page')).toBeNull();
	});

	it('clears both filters, and the page with them', async () => {
		visit('/endpoint-keys', 'provider_id=anthropic&status=active&page=2');
		const stub = stubEndpoints({ rows: threeRows() });
		await renderLoaded(stub);

		await fireEvent.click(screen.getByRole('button', { name: 'Clear filters' }));

		await waitFor(() => {
			expect(pageState.url.searchParams.get('provider_id')).toBeNull();
		});
		expect(pageState.url.searchParams.get('status')).toBeNull();
		expect(pageState.url.searchParams.get('page')).toBeNull();
	});

	it('keeps a parameter that is not a filter, so clearing filters does not close the create form', async () => {
		visit('/endpoint-keys', 'provider=openai&status=active');
		stubEndpoints({ rows: threeRows() });
		await renderTab();

		await fireEvent.click(screen.getByRole('button', { name: 'Clear filters' }));

		await waitFor(() => {
			expect(pageState.url.searchParams.get('status')).toBeNull();
		});
		expect(pageState.url.searchParams.get('provider')).toBe('openai');
	});

	it('pages on the server and keeps the page in the URL', async () => {
		const rows = Array.from({ length: 30 }, (_, index) =>
			endpointRow({ id: `ep_${index + 1}`, label: `Endpoint ${index + 1}`, priority: index + 1 })
		);
		const stub = stubEndpoints({ rows });
		await renderTab();

		expect(screen.getByText('Page 1 of 2')).toBeTruthy();
		expect(within(table()).queryByText('Endpoint 30')).toBeNull();

		await fireEvent.click(screen.getByRole('button', { name: 'Next' }));

		await waitFor(() => {
			expect(pageState.url.searchParams.get('page')).toBe('2');
		});
		await waitFor(() => {
			expect(within(table()).getByText('Endpoint 30')).toBeTruthy();
		});
		expect(within(table()).queryByText('Endpoint 1')).toBeNull();
		expect(pageQuery(stub).get('page')).toBe('2');
	});

	it('re-reads the page on screen when the operator asks for it, filters and all', async () => {
		visit('/endpoint-keys', 'provider_id=anthropic&status=disabled');
		const stub = stubEndpoints({ rows: threeRows() });
		await renderLoaded(stub);

		const before = stub.requested.length;
		await fireEvent.click(screen.getByRole('button', { name: 'Refresh now' }));

		await waitFor(() => expect(stub.requested.length).toBeGreaterThan(before));
		// §8.6.2: the control repeats the read the screen is showing. A read without the filters would swap
		// the narrowed page for the whole registry while the URL still said otherwise.
		expect(pageQuery(stub).get('provider_id')).toBe('anthropic');
		expect(pageQuery(stub).get('status')).toBe('disabled');
	});
});
