// Gateway key row actions (docs/SPEC-UI/001-SPEC-UI.md §6.2, tab 1): rename, disable, revoke.
//
// These are the actions the row owns, and two of them were broken against the served wire before this pass:
// the update route answers with the key row and no plaintext at all, while the panel parsed the create
// shape and demanded one, so every rename and toggle failed a parse the row then discarded. A refusal is
// therefore part of what is tested here, not only the happy path: a rename the gateway rejects has to stay
// in the field with the gateway's message, and a revocation that was refused has to say so instead of
// looking like it worked.

import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import GatewayKeysTab from '../../src/lib/components/GatewayKeysTab.svelte';
import { keyRow, stubGatewayKeys } from '../support/gateway-key-stub';

/** The table row that carries the given name. */
async function rowFor(name: string): Promise<HTMLElement> {
	return (await screen.findByText(name)).closest('tr') as HTMLElement;
}

async function startRename(name: string, next: string): Promise<void> {
	await fireEvent.click(within(await rowFor(name)).getByRole('button', { name: 'Rename' }));
	await fireEvent.input(screen.getByLabelText('New name'), { target: { value: next } });
	await fireEvent.click(screen.getByRole('button', { name: 'Save' }));
}

describe('gateway key row actions', () => {
	afterEach(() => {
		cleanup();
		vi.unstubAllGlobals();
	});

	it('renames a key through the update route', async () => {
		const stub = stubGatewayKeys({ update: { status: 200, body: keyRow({ name: 'Laptop two' }) } });
		render(GatewayKeysTab);
		await screen.findByRole('table');

		await startRename('Laptop', 'Laptop two');

		await waitFor(() => expect(stub.writes).toHaveLength(1));
		expect(stub.writes[0].method).toBe('PATCH');
		expect(stub.writes[0].url).toBe('/api/v1/gateway-keys/gky_1');
		expect(stub.writes[0].body).toEqual({ name: 'Laptop two' });
	});

	it('keeps the draft and the gateway message when a rename is refused', async () => {
		stubGatewayKeys({
			update: {
				status: 409,
				body: { error: { code: 'CONFLICT', message: 'A key with that name already exists.' } }
			}
		});
		render(GatewayKeysTab);
		await screen.findByRole('table');

		await startRename('Laptop', 'Laptop two');

		expect(await screen.findByRole('alert')).toBeTruthy();
		expect(screen.getByRole('alert').textContent).toBe('A key with that name already exists.');
		// The field is still open with the draft in it, so the operator can fix the name.
		expect((screen.getByLabelText('New name') as HTMLInputElement).value).toBe('Laptop two');
	});

	it('disables an active key through the same route, with the status it is moving to', async () => {
		const stub = stubGatewayKeys({ update: { status: 200, body: keyRow({ status: 'disabled' }) } });
		render(GatewayKeysTab);
		await screen.findByRole('table');

		await fireEvent.click(within(await rowFor('Laptop')).getByRole('button', { name: 'Disable' }));

		await waitFor(() => expect(stub.writes).toHaveLength(1));
		expect(stub.writes[0].body).toEqual({ status: 'disabled' });
	});

	it('confirms a revocation by name before sending it', async () => {
		const stub = stubGatewayKeys();
		render(GatewayKeysTab);
		await screen.findByRole('table');

		await fireEvent.click(within(await rowFor('Laptop')).getByRole('button', { name: 'Revoke' }));

		const dialog = await screen.findByRole('dialog');
		expect(within(dialog).getByText('Laptop')).toBeTruthy();
		expect(dialog.textContent).toContain('sk-…abcd');
		// Nothing is sent until the confirmation is answered.
		expect(stub.writes).toHaveLength(0);

		await fireEvent.click(within(dialog).getByRole('button', { name: 'Revoke key' }));

		await waitFor(() => expect(stub.writes).toHaveLength(1));
		expect(stub.writes[0].method).toBe('DELETE');
		expect(stub.writes[0].url).toBe('/api/v1/gateway-keys/gky_1');
	});

	it('reports a revocation the gateway refused, instead of looking like it worked', async () => {
		stubGatewayKeys({
			revoke: {
				status: 409,
				body: { error: { code: 'CONFLICT', message: 'That key is the last one.' } }
			}
		});
		render(GatewayKeysTab);
		await screen.findByRole('table');

		await fireEvent.click(within(await rowFor('Laptop')).getByRole('button', { name: 'Revoke' }));
		await fireEvent.click(
			within(await screen.findByRole('dialog')).getByRole('button', { name: 'Revoke key' })
		);

		expect(await screen.findByRole('alert')).toBeTruthy();
		expect(screen.getByRole('alert').textContent).toBe('That key is the last one.');
	});
});
