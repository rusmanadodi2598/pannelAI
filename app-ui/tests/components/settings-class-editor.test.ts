// The failure-class table the Routing tab saves (docs/SPEC-API/001-SPEC-API.md §7.5, §7.14).
//
// A vendor's 429 is two different failures, a throttle and a spent allowance, and the class table is where
// an operator tells them apart. The assertions are on the PATCH body because the rule that matters is
// which classes and fields the tab names: the gateway merges what a body carries and leaves the rest
// where it is, so an entry the operator never set must not be on the wire.

import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import SettingsPage from '../../src/routes/settings/+page.svelte';
import { classRow, openTab, stubSettings } from '../support/settings-stub';
import { routingGroup, settingsDocument } from '../support/settings-document';
import { value } from '../support/dom';

vi.mock('$app/paths', () => ({ resolve: (path: string) => path }));

describe('the routing tab failure-class editor', () => {
	afterEach(() => {
		cleanup();
		vi.unstubAllGlobals();
	});

	it('offers the six classes that implicate a credential, and none of the three that do not', async () => {
		stubSettings();
		render(SettingsPage);
		await openTab('Routing');

		for (const title of [
			'Temporary rate limit',
			'Quota exhausted',
			'Rejected credential',
			'Billing exhausted',
			'Upstream outage',
			'Unrecognised failure'
		]) {
			expect(await classRow(title), title).toBeTruthy();
		}

		for (const title of ['Unreachable target', 'Blocked at the upstream gate', 'Invalid request']) {
			expect(screen.queryByRole('group', { name: title }), title).toBeNull();
		}
	});

	it('shows what the gateway stored, and inherits the classes it did not', async () => {
		stubSettings({
			settings: settingsDocument({
				routing: routingGroup({
					failure_classes: { QUOTA_EXHAUSTED: { action: 'park', ceiling_seconds: 3600 } }
				})
			})
		});
		render(SettingsPage);
		await openTab('Routing');

		const stored = await classRow('Quota exhausted');
		expect(value(within(stored).getByLabelText('Action'))).toBe('park');
		expect(value(within(stored).getByLabelText('Ceiling (seconds)'))).toBe('3600');

		const inherited = await classRow('Billing exhausted');
		expect(value(within(inherited).getByLabelText('Action'))).toBe('');
		expect(value(within(inherited).getByLabelText('Ceiling (seconds)'))).toBe('');
	});

	it('sends the class and the field the operator set, and no class nobody set', async () => {
		const stub = stubSettings();
		render(SettingsPage);
		await openTab('Routing');

		const row = await classRow('Quota exhausted');
		await fireEvent.input(within(row).getByLabelText('Ceiling (seconds)'), {
			target: { value: '86400' }
		});
		await fireEvent.click(screen.getByRole('button', { name: 'Save routing settings' }));

		await waitFor(() => {
			expect(stub.patches.length).toBe(1);
		});

		expect(stub.patches[0].routing).toEqual({
			combo_strategy: 'fallback',
			combo_sticky_limit: 1,
			sticky_limit: 3,
			fallback_strategy: 'fill-first',
			key_park: {
				auth_seconds: 120,
				rate_limit_seconds: 60,
				transient_seconds: 15,
				billing_seconds: 900
			},
			failover: { total_ms: 30000, max_attempts: 8 },
			failure_classes: { QUOTA_EXHAUSTED: { ceiling_seconds: 86400 } }
		});
	});

	it('leaves the class table off the body entirely when no class is named', async () => {
		const stub = stubSettings();
		render(SettingsPage);
		await openTab('Routing');

		await fireEvent.input(screen.getByLabelText('Routing sticky limit'), {
			target: { value: '5' }
		});
		await fireEvent.click(screen.getByRole('button', { name: 'Save routing settings' }));

		await waitFor(() => {
			expect(stub.patches.length).toBe(1);
		});

		expect('failure_classes' in (stub.patches[0].routing as Record<string, unknown>)).toBe(false);
	});

	it('names no class for an entry the operator blanked again', async () => {
		const stub = stubSettings({
			settings: settingsDocument({
				routing: routingGroup({ failure_classes: { AUTH_INVALID: { action: 'rotate' } } })
			})
		});
		render(SettingsPage);
		await openTab('Routing');

		const row = await classRow('Rejected credential');
		await fireEvent.change(within(row).getByLabelText('Action'), { target: { value: '' } });
		await fireEvent.click(screen.getByRole('button', { name: 'Save routing settings' }));

		await waitFor(() => {
			expect(stub.patches.length).toBe(1);
		});

		expect('failure_classes' in (stub.patches[0].routing as Record<string, unknown>)).toBe(false);
	});

	it('takes the two windows away from a class the operator set to stop', async () => {
		stubSettings();
		render(SettingsPage);
		await openTab('Routing');

		const row = await classRow('Temporary rate limit');
		await fireEvent.change(within(row).getByLabelText('Action'), { target: { value: 'stop' } });

		// A class that stops the walk withholds no credential, so there is no wait to bound. The gateway
		// reads no window for it, and a control that cannot change the save is not on screen.
		expect(within(row).queryByLabelText('Ceiling (seconds)')).toBeNull();
		expect(within(row).queryByLabelText('Flat cooldown (seconds)')).toBeNull();
	});

	it('refuses a ceiling past the bound without sending anything', async () => {
		const stub = stubSettings();
		render(SettingsPage);
		await openTab('Routing');

		const row = await classRow('Upstream outage');
		await fireEvent.input(within(row).getByLabelText('Ceiling (seconds)'), {
			target: { value: '2592001' }
		});
		await fireEvent.click(screen.getByRole('button', { name: 'Save routing settings' }));

		expect(await screen.findByText(/must be 2592000 seconds or fewer/)).toBeTruthy();
		expect(stub.patches.length).toBe(0);
	});

	it('goes quiet again when a typed ceiling is blanked back to inherit', async () => {
		stubSettings();
		render(SettingsPage);
		await openTab('Routing');

		const row = await classRow('Unrecognised failure');
		const ceiling = within(row).getByLabelText('Ceiling (seconds)');
		await fireEvent.input(ceiling, { target: { value: '90' } });
		expect(screen.getByText('Unsaved changes')).toBeTruthy();

		// Blank is inherit, and an entry left with no field in it is held as absent rather than as an
		// empty object, or the tab would claim an unsaved change that has nothing to save.
		await fireEvent.input(ceiling, { target: { value: '' } });
		expect(screen.queryByText('Unsaved changes')).toBeNull();
	});
});
