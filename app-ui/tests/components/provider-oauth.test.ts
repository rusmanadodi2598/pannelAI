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

import { cleanup, render, screen, waitFor, within } from '@testing-library/svelte';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import ProviderDetailPage from '../../src/routes/providers/[provider_id]/+page.svelte';
import { oauthEndpointRow, stubModels, type ModelStub } from '../support/model-stub';
import { pageState, visit } from '../support/page.svelte';
import { squashed } from '../support/dom';

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

function renderProvider(): void {
	render(ProviderDetailPage, { props: { params: { provider_id: 'xai' }, data: {} } });
}

async function waitForAccounts(): Promise<void> {
	await waitFor(() =>
		expect(screen.getByRole('table', { name: /connected oauth accounts/i })).toBeTruthy()
	);
}

function accountsTable(): HTMLElement {
	return screen.getByRole('table', { name: /connected oauth accounts/i });
}

describe('when the section appears', () => {
	it('is absent for a provider the registry does not list as OAuth', async () => {
		stub.hasOAuth = false;
		renderProvider();

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
		renderProvider();
		await waitForAccounts();

		const table = accountsTable();
		expect(within(table).getByText('xAI account')).toBeTruthy();
		expect(within(table).getByText('ep_oauth_1')).toBeTruthy();
		expect(within(table).getByText(/has expired/)).toBeTruthy();
	});

	it('says no account is connected when there is none, and still offers the start', async () => {
		stub.oauthEndpoints = [];
		renderProvider();

		await waitFor(() => expect(screen.getByText(/No OAuth account is connected/)).toBeTruthy());
		expect(screen.getByRole('button', { name: 'Start the authorization' })).toBeTruthy();
	});

	it('reports a failed read and re-reads on request', async () => {
		stub.oauthReadStatus = 500;
		renderProvider();

		await waitFor(() =>
			expect(screen.getByText(/The OAuth state could not be loaded/)).toBeTruthy()
		);
		expect(screen.queryByRole('button', { name: 'Start the authorization' })).toBeNull();

		stub.oauthReadStatus = 200;
		within(screen.getByRole('alert')).getByRole('button', { name: 'Try again' }).click();

		await waitForAccounts();
	});
});

describe('the start action', () => {
	it('sends the start and links to the authorize URL rather than navigating to it', async () => {
		renderProvider();
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
		renderProvider();
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
		{ flow: 'device', contains: 'device endpoint instead' },
		{ flow: 'connector', contains: 'needs a connector' },
		{ flow: 'none', contains: 'no authorize URL' }
	]) {
		it(`states the reason for ${testCase.flow} instead of offering a control`, async () => {
			stub.oauthFlow = testCase.flow;
			renderProvider();
			await waitForAccounts();

			expect(screen.getByText(new RegExp(testCase.contains))).toBeTruthy();
			expect(screen.queryByRole('button', { name: 'Start the authorization' })).toBeNull();
		});
	}
});

describe('refreshing an account', () => {
	it('refreshes the account the row names and re-reads the state it moved', async () => {
		stub.oauthEndpoints = [oauthEndpointRow({ refresh_state: 'due' })];
		renderProvider();
		await waitForAccounts();
		expect(within(accountsTable()).getByText(/inside its refresh window/)).toBeTruthy();

		within(accountsTable()).getByRole('button', { name: 'Refresh' }).click();

		await waitFor(() => expect(stub.oauthRefreshes).toEqual(['ep_oauth_1']));
		await waitFor(() =>
			expect(screen.getByText('The gateway refreshed 1 account: ep_oauth_1.')).toBeTruthy()
		);
		// The stub moves the token, and the panel's re-read is what shows it.
		await waitFor(() =>
			expect(within(accountsTable()).getByText(/outside its refresh window/)).toBeTruthy()
		);
	});

	it('reports the refusal and leaves the row as it was', async () => {
		stub.oauthRefreshRefusal = {
			status: 400,
			code: 'VALIDATION_ERROR',
			message: 'the account has no refresh token'
		};
		renderProvider();
		await waitForAccounts();

		within(accountsTable()).getByRole('button', { name: 'Refresh' }).click();

		await waitFor(() => expect(screen.getByText(/the account has no refresh token/)).toBeTruthy());
		expect(within(accountsTable()).getByText('xAI account')).toBeTruthy();
	});

	it('offers no refresh when no account is connected, because the action belongs to a row', async () => {
		stub.oauthEndpoints = [];
		renderProvider();
		await waitFor(() => expect(screen.getByText(/No OAuth account is connected/)).toBeTruthy());
		expect(screen.queryByRole('button', { name: 'Refresh' })).toBeNull();
	});
});

describe('the callback return', () => {
	it('reports a connected account and drops the outcome from the address', async () => {
		visit('/providers/xai', 'oauth=connected&endpoint_id=ep_oauth_1');
		renderProvider();

		await waitFor(() =>
			expect(screen.getByText('The gateway connected an account (ep_oauth_1).')).toBeTruthy()
		);
		// The keys are gone from the address, so a reload does not announce a past result. The cleanup runs
		// once the status read settles, so this waits for it rather than assuming the same tick.
		await waitFor(() => expect(pageState.url.searchParams.has('oauth')).toBe(false));
		expect(pageState.url.searchParams.has('endpoint_id')).toBe(false);
		expect(pageState.url.pathname).toBe('/providers/xai');
		// The message survives the cleanup, because the section holds it rather than reading it again.
		expect(screen.getByText('The gateway connected an account (ep_oauth_1).')).toBeTruthy();
	});

	it('reports the gateway reason for a failure', async () => {
		visit(
			'/providers/xai',
			'oauth=error&oauth_error=the+state+is+unknown%2C+expired%2C+or+already+used'
		);
		renderProvider();

		await waitFor(() =>
			expect(screen.getByText(/the state is unknown, expired, or already used/)).toBeTruthy()
		);
		expect(
			squashed(screen.getByRole('alert')).startsWith('The authorization did not complete.')
		).toBe(true);
	});

	it('renders nothing about OAuth for an outcome the panel does not know, and still cleans up', async () => {
		visit('/providers/xai', 'oauth=pending&page=2');
		renderProvider();

		await waitForAccounts();
		expect(screen.queryByText(/The authorization did not complete/)).toBeNull();
		expect(screen.queryByText(/The gateway connected an account/)).toBeNull();
		// The keys are the panel's, so they are dropped even when the value was not understood; a query key
		// that belongs to something else is left alone.
		await waitFor(() => expect(pageState.url.searchParams.has('oauth')).toBe(false));
		expect(pageState.url.searchParams.get('page')).toBe('2');
	});
});
