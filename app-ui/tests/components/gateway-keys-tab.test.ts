// Gateway keys tab render tests (docs/SPEC-UI/001-SPEC-UI.md §6.2, tab 1): the list and the create flow.
//
// The tab had no render test, and the stub answers with the shapes `app-serv` actually serves rather than
// the shapes the panel hoped for. Two mismatches fell out of that and are fixed in the same change: the
// create response names the plaintext key `plaintext_key`, and a key that was never used omits
// `last_used_at` (Go's `omitempty` on a nil pointer). Before the fix every create failed its parse, so the
// one-time key never reached the modal.
//
// The row actions are their own file: rename, disable, and delete are a different concern from the list
// and the one-time key, and together they took the file past the panel's line limit.
//
// The copy cases belong to the one-time key: the modal is the only place the plaintext exists, so a copy
// control that reports nothing on a refused write loses the one value the gateway will never show again.

import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import GatewayKeysTab from '../../src/lib/components/GatewayKeysTab.svelte';
import { keyRow, stubGatewayKeys } from '../support/gateway-key-stub';
import { expectIconOnly } from '../support/icon-only';

async function createKey(name: string): Promise<void> {
	await fireEvent.input(screen.getByLabelText('Key name'), { target: { value: name } });
	await fireEvent.submit(
		screen.getByRole('button', { name: 'Create gateway key' }).closest('form')!
	);
}

function stubClipboard(writeText: (value: string) => Promise<void>): void {
	Object.defineProperty(navigator, 'clipboard', { value: { writeText }, configurable: true });
}

describe('gateway keys tab', () => {
	afterEach(() => {
		cleanup();
		vi.unstubAllGlobals();
		delete (navigator as { clipboard?: unknown }).clipboard;
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

	it('hides a key the gateway has already revoked, so a deleted key leaves the screen', async () => {
		stubGatewayKeys({
			rows: [
				keyRow(),
				keyRow({
					id: 'gky_2',
					name: 'Old laptop',
					status: 'revoked',
					revoked_at: '2026-09-24T15:54:40Z'
				})
			]
		});

		render(GatewayKeysTab);

		expect(await screen.findByRole('table')).toBeTruthy();
		expect(screen.getByText('Laptop')).toBeTruthy();
		expect(screen.queryByText('Old laptop')).toBeNull();
	});

	it('says the page holds no keys when every key on it was deleted, and keeps the paging', async () => {
		stubGatewayKeys({ rows: [keyRow({ status: 'revoked' })] });

		render(GatewayKeysTab);

		expect(await screen.findByText('No keys on this page')).toBeTruthy();
		expect(screen.queryByRole('table')).toBeNull();
		// The paging controls stay, because the operator may need to leave this page.
		expect(screen.getByRole('button', { name: 'Previous' })).toBeTruthy();
	});

	it('reports a failed list read and retries it on request', async () => {
		stubGatewayKeys({ listStatus: 500 });

		render(GatewayKeysTab);

		expect(await screen.findByText('Gateway keys could not be loaded')).toBeTruthy();
		expect(screen.getByText('the list is unavailable')).toBeTruthy();
		expect(screen.getByRole('button', { name: 'Try again' })).toBeTruthy();
	});

	it('re-reads the list when the operator asks for it', async () => {
		const stub = stubGatewayKeys();
		render(GatewayKeysTab);
		await screen.findByRole('table');
		const before = stub.reads.length;

		await fireEvent.click(screen.getByRole('button', { name: 'Refresh now' }));

		// §8.6.2: the control re-reads the list in place, because reloading the page is the only other way
		// to find out whether a key that just failed is working again.
		await waitFor(() => expect(stub.reads.length).toBeGreaterThan(before));
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

	it('copies the new key, and unlocks dismissal once it reached the clipboard', async () => {
		const copied: string[] = [];
		stubClipboard(async (value) => {
			copied.push(value);
		});
		stubGatewayKeys();
		render(GatewayKeysTab);
		await screen.findByRole('table');

		await createKey('CI runner');
		const dialog = await screen.findByRole('dialog');

		// The copy control is icon-only here (owner directive, 2026-09-24), so the button carries the name
		// the glyph cannot.
		const copy = within(dialog).getByRole('button', { name: 'Copy' });
		expectIconOnly(copy, 'Copy');

		await fireEvent.click(copy);

		expect(copied).toEqual(['sk-live-once-9999']);
		expect(within(dialog).getByText('Copied.')).toBeTruthy();
		// §6.2: the modal closes with an explicit control or Escape after the copy control reports success.
		expect(within(dialog).getByRole('button', { name: 'Close dialog' })).toBeTruthy();
	});

	it('says a refused copy failed, and stays gated on the acknowledgement', async () => {
		// The clipboard is absent outside a secure context, which is where the panel is often opened. The
		// copy then threw with nothing on screen, so the button read as dead and the key looked lost.
		stubGatewayKeys();
		render(GatewayKeysTab);
		await screen.findByRole('table');

		await createKey('CI runner');
		const dialog = await screen.findByRole('dialog');

		await fireEvent.click(within(dialog).getByRole('button', { name: 'Copy' }));

		expect(within(dialog).getByText('Copy failed. Select the text and copy it.')).toBeTruthy();
		// A refused copy is not a reason to let the one value the gateway will never show again be closed
		// away, so the gate stays where it was.
		expect(within(dialog).queryByRole('button', { name: 'Close dialog' })).toBeNull();
		expect(
			(within(dialog).getByRole('button', { name: 'Done' }) as HTMLButtonElement).disabled
		).toBe(true);
	});
});
