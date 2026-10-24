// What a refresh and a callback return leave on the OAuth section (docs/SPEC-UI/001-SPEC-UI.md §6.3).
//
// Both are reads the panel makes after the vendor has answered: a refresh moves a provider's endpoints onto a
// live token, and a callback carries the code back through the page's own query. The assertions are on the URL
// the panel produced rather than on a call it made.
//
// The page is the unit under test, as with the other provider detail sections, because the section renders
// from a provider fact (`has_oauth`) that the page owns. The callback return is read from the page's own
// query, so the `$app/state` and `$app/navigation` mocks write to the shared reactive stand-in.

import { cleanup, screen, waitFor, within } from '@testing-library/svelte';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { oauthEndpointRow, stubModels, type ModelStub } from '../support/model-stub';
import { pageState, visit } from '../support/page.svelte';
import { squashed } from '../support/dom';
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

describe('refreshing an account', () => {
	it('refreshes the account the row names and re-reads the state it moved', async () => {
		stub.oauthEndpoints = [oauthEndpointRow({ refresh_state: 'due' })];
		await renderProvider();
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
		await renderProvider();
		await waitForAccounts();

		within(accountsTable()).getByRole('button', { name: 'Refresh' }).click();

		await waitFor(() => expect(screen.getByText(/the account has no refresh token/)).toBeTruthy());
		expect(within(accountsTable()).getByText('xAI account')).toBeTruthy();
	});

	it('offers no refresh when no account is connected, because the action belongs to a row', async () => {
		stub.oauthEndpoints = [];
		await renderProvider();
		await waitFor(() => expect(screen.getByText(/No OAuth account is connected/)).toBeTruthy());
		expect(screen.queryByRole('button', { name: 'Refresh' })).toBeNull();
	});
});

describe('the callback return', () => {
	it('reports a connected account and drops the outcome from the address', async () => {
		visit('/providers/xai', 'oauth=connected&endpoint_id=ep_oauth_1');
		await renderProvider();

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
		await renderProvider();

		await waitFor(() =>
			expect(screen.getByText(/the state is unknown, expired, or already used/)).toBeTruthy()
		);
		expect(
			squashed(screen.getByRole('alert')).startsWith('The authorization did not complete.')
		).toBe(true);
	});

	it('renders nothing about OAuth for an outcome the panel does not know, and still cleans up', async () => {
		visit('/providers/xai', 'oauth=pending&page=2');
		await renderProvider();

		await waitForAccounts();
		expect(screen.queryByText(/The authorization did not complete/)).toBeNull();
		expect(screen.queryByText(/The gateway connected an account/)).toBeNull();
		// The keys are the panel's, so they are dropped even when the value was not understood; a query key
		// that belongs to something else is left alone.
		await waitFor(() => expect(pageState.url.searchParams.has('oauth')).toBe(false));
		expect(pageState.url.searchParams.get('page')).toBe('2');
	});
});
