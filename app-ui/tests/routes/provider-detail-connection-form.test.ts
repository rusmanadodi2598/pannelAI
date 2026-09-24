// The endpoint form inside a provider's Connections section (docs/SPEC-UI/001-SPEC-UI.md §6.3).
//
// A provider whose auth type takes no key has no Add API Key button, so this form is its only way to a
// connection. Since 2026-09-24 it renders here instead of on the Endpoint & Key page, which no longer
// carries upstream endpoints at all, so what these cases hold is the path from this screen to the wire
// and back into the table below it: one POST, the row the API stored, and the form closed behind it.
//
// The sibling file `provider-detail-key-dialog.test.ts` covers the key dialog, which stays the path for
// the auth types that carry a credential.

import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import ProviderDetailPage from '../../src/routes/providers/[provider_id]/+page.svelte';
import { stubModels } from '../support/model-stub';

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
	render(ProviderDetailPage, { props: { params: { provider_id: 'openai' }, data: {} } });
}

/** Types into a field by its label, through the event Svelte 5's `bind:value` listens for. */
async function fill(label: string, text: string): Promise<void> {
	await fireEvent.input(screen.getByLabelText(label), { target: { value: text } });
}

async function openForm(): Promise<void> {
	await fireEvent.click(await screen.findByRole('button', { name: 'Add a connection' }));
	await screen.findByText('Add an endpoint for openai');
}

describe('the endpoint form on a provider', () => {
	it('opens and closes from the same control, so the section is not permanently taller', async () => {
		stubModels({ authType: 'oauth' });
		renderProvider();

		await openForm();
		expect(screen.getByLabelText('Label')).toBeTruthy();

		await fireEvent.click(screen.getByRole('button', { name: 'Hide the form' }));
		expect(screen.queryByLabelText('Label')).toBeNull();
	});

	it('stores the endpoint under the auth type the operator picked and lists it', async () => {
		const stub = stubModels({ authType: 'oauth' });
		renderProvider();
		await openForm();

		await fill('Label', 'Primary');
		await fireEvent.change(screen.getByLabelText('Auth type'), { target: { value: 'oauth' } });
		await fill('Priority (optional)', '4');
		await fireEvent.click(screen.getByRole('button', { name: 'Add endpoint' }));

		await waitFor(() => expect(stub.endpointCreates).toHaveLength(1));
		// The provider is fixed by the screen, so the body carries the route's own id and the form's
		// field names never reach the wire. A non-key provider sends no `keys` at all rather than an empty
		// array, because the body schema's list has a minimum of one when it is present.
		expect(stub.endpointCreates[0]).toEqual({
			provider_id: 'openai',
			label: 'Primary',
			auth_type: 'oauth',
			priority: 4
		});

		expect(await screen.findByText('Connection added.')).toBeTruthy();
		// The form closed and the section's own list re-read, which is the half a form-only test cannot hold.
		expect(screen.queryByText('Add an endpoint for openai')).toBeNull();
		const table = await screen.findByRole('table', { name: /Upstream endpoints/ });
		expect(within(table).getByText('Primary')).toBeTruthy();
	});

	it('keeps the form open with the gateway message when the create is refused', async () => {
		stubModels({ authType: 'oauth', writeStatus: 500 });
		renderProvider();
		await openForm();

		await fill('Label', 'Primary');
		await fireEvent.change(screen.getByLabelText('Auth type'), { target: { value: 'oauth' } });
		await fireEvent.click(screen.getByRole('button', { name: 'Add endpoint' }));

		expect(await screen.findByText('The endpoint could not be stored.')).toBeTruthy();
		expect(screen.getByText('Add an endpoint for openai')).toBeTruthy();
		expect(screen.queryByText('Connection added.')).toBeNull();
	});
});
