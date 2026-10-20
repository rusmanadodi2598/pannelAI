// The request detail and the purge dialog behind it (docs/SPEC-UI/001-SPEC-UI.md §6.11, §7.6).
//
// The capture states are the load-bearing part: an empty body area means three different things depending
// on whether capture is on and whether anything was stored, and only the first two are worth a screenshot.

import { cleanup, fireEvent, render, screen } from '@testing-library/svelte';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import LogsRequestsTab from '../../src/lib/components/LogsRequestsTab.svelte';
import { visit } from '../support/page.svelte';
import { detail, stubLogs } from '../support/logs-stub';
import { SvelteURLSearchParams } from 'svelte/reactivity';

vi.mock('$app/state', async () => {
	const { pageState: page } = await import('../support/page.svelte');
	return { page };
});

vi.mock('$app/paths', () => ({ resolve: (path: string) => path }));

vi.mock('$app/navigation', async () => {
	const { pageState: page, queryOf: query } = await import('../support/page.svelte');
	return {
		goto: (url: string) => {
			page.url = { pathname: '/logs', searchParams: new SvelteURLSearchParams(query(url)) };
			return Promise.resolve();
		}
	};
});

describe('the request detail and the purge dialog', () => {
	beforeEach(() => {
		visit('/logs');
	});

	afterEach(() => {
		cleanup();
		vi.unstubAllGlobals();
	});

	it('opens the request detail with its captured bodies', async () => {
		const stub = stubLogs();
		render(LogsRequestsTab);
		await screen.findByRole('table');

		await fireEvent.click(screen.getByRole('button', { name: 'Open' }));

		expect(await screen.findByText('Request body')).toBeTruthy();
		expect(screen.getByText('{"model":"gpt-4o"}')).toBeTruthy();
		expect(screen.getByText('{"choices":[]}')).toBeTruthy();
		expect(stub.requested.some((url) => url.includes('/logs/requests/req_1'))).toBe(true);
	});

	it('says capture is off, and names the fix, rather than showing an empty body area', async () => {
		stubLogs({
			detail: detail({ capture_enabled: false, request_body: undefined, response_body: undefined })
		});
		render(LogsRequestsTab);
		await screen.findByRole('table');

		await fireEvent.click(screen.getByRole('button', { name: 'Open' }));

		expect(await screen.findByText(/Capture is off/)).toBeTruthy();
		expect(screen.queryByText('Request body')).toBeNull();
	});

	it('distinguishes capture on with nothing stored from capture off', async () => {
		stubLogs({
			detail: detail({ capture_enabled: true, request_body: undefined, response_body: undefined })
		});
		render(LogsRequestsTab);
		await screen.findByRole('table');

		await fireEvent.click(screen.getByRole('button', { name: 'Open' }));

		expect(
			await screen.findByText('Capture is on, but no body was stored for this request.')
		).toBeTruthy();
		expect(screen.queryByText(/Capture is off/)).toBeNull();
	});

	it('purge requires typing the word, and states what the delete does', async () => {
		stubLogs();
		render(LogsRequestsTab);
		await screen.findByRole('table');

		await fireEvent.click(screen.getByRole('button', { name: 'Purge captured logs' }));

		// The dialog is asserted by its own copy rather than by role, because the native <dialog> element is
		// not in jsdom's accessibility tree without `showModal`, which jsdom does not implement.
		expect(
			await screen.findByText(
				'Deletes logs older than the retention window. This cannot be undone.'
			)
		).toBeTruthy();
		expect(screen.getByText(/Usage records are not affected/)).toBeTruthy();

		const confirm = screen.getByRole('button', { name: 'Purge logs' }) as HTMLButtonElement;
		expect(confirm.disabled).toBe(true);

		await fireEvent.input(screen.getByLabelText(/Type purge to confirm/), {
			target: { value: 'purge' }
		});
		expect(confirm.disabled).toBe(false);
	});
});
