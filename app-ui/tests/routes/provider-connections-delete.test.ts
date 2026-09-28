// The Connections section's delete paths and its two add affordances (docs/SPEC-UI/001-SPEC-UI.md §6.3).
//
// What this screen owns is the operator's three actions on a provider's connections: remove one, add a key
// (or Personal Access Token), and connect through OAuth. Removal is one route — `DELETE /endpoints/{id}` —
// whether the row carried an API key, a PAT, or a connected account's token, so the cases below drive all
// three surfaces (table row, detail drawer, OAuth account row) and hold that each opens the same
// confirmation, names the object, and reloads.
//
// The two buttons are the other half: a provider that declares both `oauth` and a key mode (Qoder) offers
// "Add API Key" and "Add a connection" side by side, and the key dialog stores a pasted credential as an
// `api_key` connection — not as the provider's own `oauth` auth type — because the value the operator typed
// is a static key.

import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import ProviderDetailPage from '../../src/routes/providers/[provider_id]/+page.svelte';
import { endpointRow } from '../support/endpoint-stub';
import { oauthEndpointRow, stubModels, type ModelStub } from '../support/model-stub';

vi.mock('$app/paths', () => ({
	resolve: (route: string, params?: Record<string, string>) =>
		params === undefined
			? route
			: route.replace(/\[(\w+)\]/g, (_match, key: string) => params[key] ?? `[${key}]`)
}));

afterEach(() => {
	cleanup();
	vi.unstubAllGlobals();
});

function renderProvider(): void {
	render(ProviderDetailPage, { props: { params: { provider_id: 'qoder' }, data: {} } });
}

/** Confirms the open dialog and returns once the delete is on the wire. */
async function confirmDelete(): Promise<void> {
	const dialog = await screen.findByText('Delete this connection');
	expect(dialog).toBeTruthy();
	await fireEvent.click(screen.getByRole('button', { name: 'Delete the connection' }));
}

describe('deleting a connection from its row', () => {
	it('names the endpoint, deletes it, and drops it from the list', async () => {
		const stub = stubModels({
			authType: 'api_key',
			endpoints: [endpointRow({ id: 'ep_1', label: 'Primary' })]
		});
		renderProvider();

		const table = await screen.findByRole('table', { name: /Upstream endpoints/ });
		await fireEvent.click(within(table).getByRole('button', { name: 'Delete' }));

		// The confirmation states the cascade the route performs: the endpoint's keys go with it.
		expect(await screen.findByText(/Its keys go with it/)).toBeTruthy();
		await confirmDelete();

		await waitFor(() => expect(stub.endpointDeletes).toEqual(['ep_1']));
		// Both the row and the just-closed dialog named the endpoint; after the reload neither remains.
		await waitFor(() => expect(screen.queryAllByText('Primary')).toHaveLength(0));
	}, 10_000);

	it('keeps the row and reports the refusal when the gateway declines', async () => {
		const stub: ModelStub = stubModels({
			authType: 'api_key',
			endpoints: [endpointRow({ id: 'ep_1', label: 'Primary' })],
			endpointDeleteStatus: 500
		});
		renderProvider();

		const table = await screen.findByRole('table', { name: /Upstream endpoints/ });
		await fireEvent.click(within(table).getByRole('button', { name: 'Delete' }));
		await confirmDelete();

		expect(await screen.findByText(/The connection was not deleted/)).toBeTruthy();
		expect(stub.endpointDeletes).toEqual(['ep_1']);
		expect(within(table).getByText('Primary')).toBeTruthy();
	}, 10_000);
});

describe('deleting from the detail drawer', () => {
	it('closes the drawer and opens the same confirmation', async () => {
		const stub = stubModels({
			authType: 'api_key',
			endpoints: [endpointRow({ id: 'ep_9', label: 'Secondary' })]
		});
		renderProvider();

		const table = await screen.findByRole('table', { name: /Upstream endpoints/ });
		await fireEvent.click(within(table).getByRole('button', { name: 'Open' }));

		await fireEvent.click(await screen.findByRole('button', { name: 'Delete connection' }));
		await confirmDelete();

		await waitFor(() => expect(stub.endpointDeletes).toEqual(['ep_9']));
	}, 10_000);
});

describe('deleting a connected OAuth account', () => {
	it('is the same endpoint removal, from the account row', async () => {
		const stub = stubModels({
			authType: 'oauth',
			hasOAuth: true,
			oauthFlow: 'code',
			oauthEndpoints: [oauthEndpointRow({ endpoint_id: 'ep_oauth_1', label: 'Owner account' })]
		});
		renderProvider();

		const accounts = await screen.findByRole('table', { name: /Connected OAuth accounts/ });
		await fireEvent.click(within(accounts).getByRole('button', { name: 'Delete' }));
		await confirmDelete();

		await waitFor(() => expect(stub.endpointDeletes).toEqual(['ep_oauth_1']));
	}, 10_000);
});

describe('the two add affordances on a provider that declares both', () => {
	it('offers the Personal Access Token dialog and the connection form side by side for Qoder', async () => {
		stubModels({ authType: 'oauth', hasOAuth: true, authModes: ['oauth', 'apikey'] });
		renderProvider();

		// A key + OAuth provider's key credential is a Personal Access Token, and the affordance says so.
		expect(await screen.findByRole('button', { name: 'Add Personal Access Token' })).toBeTruthy();
		expect(screen.getByRole('button', { name: 'Add a connection' })).toBeTruthy();
		expect(screen.queryByRole('button', { name: 'Add API Key' })).toBeNull();
	});

	it('stores a pasted credential as an api_key connection, not the provider oauth type', async () => {
		const stub = stubModels({ authType: 'oauth', hasOAuth: true, authModes: ['oauth', 'apikey'] });
		renderProvider();

		await fireEvent.click(await screen.findByRole('button', { name: 'Add Personal Access Token' }));
		await screen.findByLabelText('Name');
		await fireEvent.input(screen.getByLabelText('Name'), { target: { value: 'PAT' } });
		await fireEvent.input(screen.getByLabelText('Personal Access Token'), {
			target: { value: 'pt-aaaaaaaaaaaaaaaa' }
		});
		await fireEvent.click(screen.getByRole('button', { name: 'Save' }));

		await waitFor(() => expect(stub.endpointCreates).toHaveLength(1));
		expect(stub.endpointCreates[0]).toMatchObject({
			provider_id: 'qoder',
			label: 'PAT',
			auth_type: 'api_key',
			keys: [{ value: 'pt-aaaaaaaaaaaaaaaa' }]
		});
	});
});
