// The OAuth device round on the provider detail screen (docs/SPEC-UI/001-SPEC-UI.md §6.3).
//
// The device flow is the one the panel cannot hand to a browser: the gateway starts it, the operator approves
// at the vendor, and the panel polls its own read until the vendor says the code was used. So the assertions
// are about the address the panel shows, the polls it makes, and what a refusal mid-round leaves on screen.
//
// The page is the unit under test, as with the other provider detail sections, because the section renders
// from a provider fact (`has_oauth`) that the page owns.

import { cleanup, screen, waitFor } from '@testing-library/svelte';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { oauthEndpointRow, stubModels, type ModelStub } from '../support/model-stub';
import { visit } from '../support/page.svelte';
import { renderProvider, waitForAccounts } from '../support/provider-oauth-harness';

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

describe('the device round', () => {
	beforeEach(() => {
		stub.oauthFlow = 'device';
	});

	it('shows the vendor page as a link and a code, and opens nothing itself', async () => {
		await renderProvider();
		await waitForAccounts();

		screen.getByRole('button', { name: 'Start the device authorization' }).click();
		await waitFor(() => expect(stub.oauthDeviceStarts).toEqual(['xai']));

		const link = await waitFor(() => screen.getByRole('link', { name: 'Open the device page' }));
		expect(link.getAttribute('href')).toBe(stub.oauthDeviceStart.verification_url as string);
		// The address is readable without following it, which is why the link is not its only copy.
		expect(screen.getByText(stub.oauthDeviceStart.verification_url as string)).toBeTruthy();
		expect(screen.getByTestId('device-user-code').textContent).toBe('AB12CD34');
		// The round is polled with the single-use code alone: the verifier and the machine id the
		// gateway staged never reach this screen.
		expect(JSON.stringify(stub.oauthDevicePolls)).not.toContain('challenge');
	});

	// A vendor-minted state round has no short code: the screen shows the page to open and
	// waits, and does not invent a code the vendor never asked the operator to enter.
	it('renders a round that carries no short code without a code block', async () => {
		stub.oauthDeviceStart = {
			device_code: 'vendor-state-7f3a',
			verification_url: 'https://www.codebuddy.test/auth?state=vendor-state-7f3a',
			interval_seconds: 0,
			expires_in: 300
		};
		stub.oauthDevicePollAnswers = [{ status: 'pending' }];
		await renderProvider();
		await waitForAccounts();

		screen.getByRole('button', { name: 'Start the device authorization' }).click();

		const link = await waitFor(() => screen.getByRole('link', { name: 'Open the device page' }));
		expect(link.getAttribute('href')).toBe(
			'https://www.codebuddy.test/auth?state=vendor-state-7f3a'
		);
		await waitFor(() => expect(screen.getByText(/waiting for the vendor/i)).toBeTruthy());
		expect(screen.queryByText('Your code')).toBeNull();
		expect(screen.queryByTestId('device-user-code')).toBeNull();
		// The round still polls with the handle it was given, which here is the vendor's state.
		await waitFor(() => expect(stub.oauthDevicePolls.length).toBeGreaterThan(0));
		expect(
			stub.oauthDevicePolls.every((ask) => ask.device_code === 'vendor-state-7f3a')
		).toBeTruthy();
	});

	it('asks until the vendor grants the token, then re-reads the accounts', async () => {
		stub.oauthDevicePollAnswers = [
			{ status: 'pending' },
			{ status: 'connected', endpoint_id: 'ep_qoder_1', token_hint: 'dt-…7c2f', created: true }
		];
		await renderProvider();
		await waitForAccounts();

		screen.getByRole('button', { name: 'Start the device authorization' }).click();

		await waitFor(() => expect(screen.getByText(/waiting for the vendor/i)).toBeTruthy());
		await waitFor(() => expect(screen.getByText(/Connected \(ep_qoder_1\)/)).toBeTruthy());
		expect(stub.oauthDevicePolls.map((ask) => ask.device_code)).toEqual(['dev_1', 'dev_1']);
		// The connect moved a row, so the section read the state again rather than guessing it.
		await waitFor(() =>
			expect(stub.reads.filter((read) => read === 'oauth:status').length).toBeGreaterThan(1)
		);
	});

	it('reports a refused ask and stops asking', async () => {
		stub.oauthDevicePollRefusalAt = 1;
		await renderProvider();
		await waitForAccounts();

		screen.getByRole('button', { name: 'Start the device authorization' }).click();

		await waitFor(() =>
			expect(screen.getByRole('alert').textContent).toMatch(/device token endpoint refused/)
		);
		expect(screen.queryByText(/waiting for the vendor/i)).toBeNull();
		await new Promise((resolve) => setTimeout(resolve, 20));
		expect(stub.oauthDevicePolls).toHaveLength(1);
	});

	it('reports a round the gateway could not start and offers another attempt', async () => {
		stub.oauthDeviceStartRefusal = {
			status: 400,
			code: 'VALIDATION_ERROR',
			message: 'provider xai does not declare a complete device flow'
		};
		await renderProvider();
		await waitForAccounts();

		screen.getByRole('button', { name: 'Start the device authorization' }).click();

		await waitFor(() =>
			expect(screen.getByRole('alert').textContent).toMatch(/complete device flow/)
		);
		expect(screen.queryByText('Your code')).toBeNull();
	});
});
