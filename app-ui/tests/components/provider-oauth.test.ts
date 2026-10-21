// OAuth section tests (docs/SPEC-UI/001-SPEC-UI.md §6.3).
//
// The page is the unit under test, as with the other provider detail sections, because the section is
// rendered from a provider fact (`has_oauth`) that the page owns. Two things are worth asserting against
// the real render rather than against a helper: the panel offers the start action for the `code` flow and
// for nothing else, and it links to the authorize URL instead of navigating to it, which is what keeps the
// address visible before it is followed.
//
// The callback return is read from the page's own query, so the `$app/state` and `$app/navigation` mocks
// write to the shared reactive stand-in: the assertion is then on the URL the panel produced rather than on
// the fact that it called something.

import { cleanup, screen, waitFor, within } from '@testing-library/svelte';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { oauthEndpointRow, stubModels, type ModelStub } from '../support/model-stub';
import { pageState, visit } from '../support/page.svelte';
import { accountsTable, renderProvider, waitForAccounts } from '../support/provider-oauth-harness';

vi.mock('$app/state', async () => {
	const { pageState: page } = await import('../support/page.svelte');
	return { page };
});

vi.mock('$app/paths', () => ({
	resolve: (route: string, params?: Record<string, string>) =>
		params === undefined
			? route
			: route.replace(/\[(\w+)\]/g, (_match, key: string) => params[key] ?? `[${key}]`)
}));

vi.mock('$app/navigation', async () => {
	const { SvelteURLSearchParams } = await import('svelte/reactivity');
	const { pageState: page, queryOf: query } = await import('../support/page.svelte');
	return {
		goto: (url: string) => {
			const index = url.indexOf('?');
			page.url = {
				pathname: index === -1 ? url : url.slice(0, index),
				searchParams: new SvelteURLSearchParams(query(url))
			};
			return Promise.resolve();
		}
	};
});

let stub: ModelStub;

beforeEach(() => {
	visit('/providers/xai');
	stub = stubModels({
		hasOAuth: true,
		oauthFlow: 'code',
		oauthEndpoints: [oauthEndpointRow()]
	});
});

afterEach(() => {
	cleanup();
	vi.unstubAllGlobals();
});

describe('when the section appears', () => {
	it('is absent for a provider the registry does not list as OAuth', async () => {
		stub.hasOAuth = false;
		await renderProvider('Models');

		await waitFor(() => expect(screen.getByText('Model catalog')).toBeTruthy());
		expect(screen.queryByRole('button', { name: 'Start the authorization' })).toBeNull();
		expect(stub.reads).not.toContain('oauth:status');
	});

	it('lists the connected accounts with the state the gateway derives', async () => {
		stub.oauthEndpoints = [
			oauthEndpointRow({
				refresh_state: 'due',
				expires_at: new Date(Date.now() - 60 * 60 * 1000).toISOString()
			})
		];
		await renderProvider();
		await waitForAccounts();

		const table = accountsTable();
		expect(within(table).getByText('xAI account')).toBeTruthy();
		expect(within(table).getByText('ep_oauth_1')).toBeTruthy();
		expect(within(table).getByText(/has expired/)).toBeTruthy();
	});

	it('says no account is connected when there is none, and still offers the start', async () => {
		stub.oauthEndpoints = [];
		await renderProvider();

		await waitFor(() => expect(screen.getByText(/No OAuth account is connected/)).toBeTruthy());
		expect(screen.getByRole('button', { name: 'Start the authorization' })).toBeTruthy();
	});

	it('reports a failed read and re-reads on request', async () => {
		stub.oauthReadStatus = 500;
		await renderProvider();

		await waitFor(() => expect(screen.getByText(/Could not load the OAuth state/)).toBeTruthy());
		expect(screen.queryByRole('button', { name: 'Start the authorization' })).toBeNull();

		stub.oauthReadStatus = 200;
		within(screen.getByRole('alert')).getByRole('button', { name: 'Try again' }).click();

		await waitForAccounts();
	});
});

describe('the start action', () => {
	it('sends the start and links to the authorize URL rather than navigating to it', async () => {
		await renderProvider();
		await waitForAccounts();

		screen.getByRole('button', { name: 'Start the authorization' }).click();

		await waitFor(() => expect(stub.oauthStarts).toEqual(['xai']));
		const link = await screen.findByRole('link', { name: 'Open the authorization page' });
		expect(link.getAttribute('href')).toBe('https://provider.test/authorize?state=st_1');
		// The panel is still on the provider page: the navigation is the operator's to make.
		expect(pageState.url.pathname).toBe('/providers/xai');
	});

	it('reports the refusal when a state is already in flight, and links nowhere', async () => {
		stub.oauthStartRefusal = {
			status: 409,
			code: 'CONFLICT',
			message: 'an oauth state is already in flight'
		};
		await renderProvider();
		await waitForAccounts();

		screen.getByRole('button', { name: 'Start the authorization' }).click();

		await waitFor(() =>
			expect(screen.getByText(/an oauth state is already in flight/)).toBeTruthy()
		);
		expect(screen.queryByRole('link', { name: 'Open the authorization page' })).toBeNull();
	});
});

describe('the flows the panel cannot start', () => {
	for (const testCase of [
		{ flow: 'connector', contains: 'needs a connector' },
		{ flow: 'none', contains: 'no authorize URL' }
	]) {
		it(`states the reason for ${testCase.flow} instead of offering a control`, async () => {
			stub.oauthFlow = testCase.flow;
			await renderProvider();
			await waitForAccounts();

			expect(screen.getByText(new RegExp(testCase.contains))).toBeTruthy();
			expect(screen.queryByRole('button', { name: 'Start the authorization' })).toBeNull();
		});
	}
});
