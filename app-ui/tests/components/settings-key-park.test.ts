// The key-park windows the Routing tab edits (docs/SPEC-API/001-SPEC-API.md §7.14).
//
// The group is a component now, and the one thing worth pinning is how a window reaches the tab's draft:
// the component writes back a whole `key_park` object rather than mutating a field in place. A write that
// did not land would still show the typed number, because the input keeps what the operator typed, and the
// save would send what was there before.

import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import SettingsPage from '../../src/routes/settings/+page.svelte';
import { openTab, stubSettings } from '../support/settings-stub';

vi.mock('$app/paths', () => ({ resolve: (path: string) => path }));

describe('the routing tab key-park windows', () => {
	afterEach(() => {
		cleanup();
		vi.unstubAllGlobals();
	});

	it('shows the windows the gateway stored', async () => {
		stubSettings();
		render(SettingsPage);
		await openTab('Routing');

		expect((screen.getByLabelText('Auth park window') as HTMLInputElement).value).toBe('120');
		expect((screen.getByLabelText('Billing park window') as HTMLInputElement).value).toBe('900');
	});

	it('carries a retyped window into the body the tab sends', async () => {
		const stub = stubSettings();
		render(SettingsPage);
		await openTab('Routing');

		await fireEvent.input(screen.getByLabelText('Auth park window'), { target: { value: '300' } });
		await fireEvent.click(screen.getByRole('button', { name: 'Save routing settings' }));

		await waitFor(() => {
			expect(stub.patches.length).toBe(1);
		});

		expect(stub.patches[0].routing).toMatchObject({ key_park: { auth_seconds: 300 } });
	});

	it('sends no window for one the operator blanked, which is what inherit means', async () => {
		const stub = stubSettings();
		render(SettingsPage);
		await openTab('Routing');

		await fireEvent.input(screen.getByLabelText('Transient park window'), {
			target: { value: '' }
		});
		await fireEvent.click(screen.getByRole('button', { name: 'Save routing settings' }));

		await waitFor(() => {
			expect(stub.patches.length).toBe(1);
		});

		const park = (stub.patches[0].routing as Record<string, unknown>).key_park as Record<
			string,
			unknown
		>;
		expect('transient_seconds' in park).toBe(false);
		expect(park.auth_seconds).toBe(120);
	});

	it('refuses a window past its ceiling without sending anything', async () => {
		const stub = stubSettings();
		render(SettingsPage);
		await openTab('Routing');

		await fireEvent.input(screen.getByLabelText('Rate-limit park window'), {
			target: { value: '301' }
		});
		await fireEvent.click(screen.getByRole('button', { name: 'Save routing settings' }));

		expect(
			await screen.findByText('The rate-limit park window must be 300 seconds or fewer.')
		).toBeTruthy();
		expect(stub.patches.length).toBe(0);
	});
});
