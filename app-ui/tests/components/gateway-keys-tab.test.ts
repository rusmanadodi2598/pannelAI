// Gateway keys tab render tests (docs/SPEC-UI/001-SPEC-UI.md §6.2, tab 1): the list and the create flow.
//
// The tab had no render test, and the stub answers with the shapes `app-serv` actually serves rather than
// the shapes the panel hoped for. Two mismatches fell out of that and are fixed in the same change: the
// create response names the plaintext key `plaintext_key`, and a key that was never used omits
// `last_used_at` (Go's `omitempty` on a nil pointer). Before the fix every create failed its parse, so the
// one-time key never reached the modal.
//
// The row actions are their own file: rename, disable, and revoke are a different concern from the list and
// the one-time key, and together they took the file past the panel's line limit.

import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import GatewayKeysTab from '../../src/lib/components/GatewayKeysTab.svelte';
import { keyRow, stubGatewayKeys } from '../support/gateway-key-stub';

async function createKey(name: string): Promise<void> {
	await fireEvent.input(screen.getByLabelText('Key name'), { target: { value: name } });
	await fireEvent.submit(
		screen.getByRole('button', { name: 'Create gateway key' }).closest('form')!
	);
}

describe('gateway keys tab', () => {
	afterEach(() => {
		cleanup();
		vi.unstubAllGlobals();
	});

	it('renders a row per key, with "Never" for a key that has not been used', async () => {
		stubGatewayKeys({
			rows: [keyRow(), keyRow({ id: 'gky_2', name: 'CI', key_hint: 'sk-…efgh', request_count: 0 })]
		});

		render(GatewayKeysTab);

		expect(await screen.findByRole('table')).toBeTruthy();
		expect(screen.getByText('Laptop')).toBeTruthy();
		expect(screen.getByText('sk-…abcd')).toBeTruthy();
		expect(screen.getByText('12')).toBeTruthy();
		expect(screen.getAllByText('Never')).toHaveLength(2);
	});

	it('says the list is empty rather than showing an empty table', async () => {
		stubGatewayKeys({ rows: [] });

		render(GatewayKeysTab);

		expect(await screen.findByText('No gateway keys yet')).toBeTruthy();
		expect(screen.queryByRole('table')).toBeNull();
	});

	it('reports a failed list read and retries it on request', async () => {
		stubGatewayKeys({ listStatus: 500 });

		render(GatewayKeysTab);

		expect(await screen.findByText('Gateway keys could not be loaded')).toBeTruthy();
		expect(screen.getByText('the list is unavailable')).toBeTruthy();
		expect(screen.getByRole('button', { name: 'Try again' })).toBeTruthy();
	});

	it('shows a new key once, and will not let it be dismissed before it is acknowledged', async () => {
		const stub = stubGatewayKeys();
		render(GatewayKeysTab);
		await screen.findByRole('table');

		await createKey('CI runner');

		const dialog = await screen.findByRole('dialog');
		expect(within(dialog).getByText('sk-live-once-9999')).toBeTruthy();
		expect(
			within(dialog).getByText(
				'This key is shown once. Store it now, because the gateway keeps only a hash of it.'
			)
		).toBeTruthy();

		// §6.2: the modal closes with an explicit control after the acknowledgement is ticked, and the close
		// control is not offered before that.
		expect(
			(within(dialog).getByRole('button', { name: 'Done' }) as HTMLButtonElement).disabled
		).toBe(true);
		expect(within(dialog).queryByRole('button', { name: 'Close dialog' })).toBeNull();

		await fireEvent.click(within(dialog).getByRole('checkbox'));

		const done = within(dialog).getByRole('button', { name: 'Done' }) as HTMLButtonElement;
		expect(done.disabled).toBe(false);
		await fireEvent.click(done);

		await waitFor(() => expect(screen.queryByText('sk-live-once-9999')).toBeNull());
		expect(stub.writes[0]).toEqual({
			method: 'POST',
			url: '/api/v1/gateway-keys',
			body: { name: 'CI runner' }
		});
	});

	it('refuses an empty name in the panel, without asking the gateway', async () => {
		const stub = stubGatewayKeys();
		render(GatewayKeysTab);
		await screen.findByRole('table');

		await createKey('');

		expect(await screen.findByRole('alert')).toBeTruthy();
		expect(stub.writes).toHaveLength(0);
	});
});
