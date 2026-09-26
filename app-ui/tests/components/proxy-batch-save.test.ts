// Batch add and outbound settings write tests (docs/SPEC-UI/001-SPEC-UI.md §6.9).
//
// Two rules live here that the screen cannot show by rendering: the paste is submitted one row at a
// time in order, and a run that only partly succeeds keeps the lines that were refused so pressing
// the button again cannot add a duplicate of a row that already landed. The second one is the reason
// the box is not simply cleared on completion.

import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import ProxyPoolsPage from '../../src/routes/proxy-pools/+page.svelte';
import { text, value } from '../support/dom';
import { stubProxies } from '../support/proxy-stub';
import { settingsDocument } from '../support/settings-document';

async function openBatch(paste: string): Promise<void> {
	await screen.findByText('No proxies yet');
	await fireEvent.click(screen.getByRole('button', { name: 'Add several at once' }));
	await fireEvent.input(screen.getByLabelText('Proxy URLs, one per line'), {
		target: { value: paste }
	});
}

describe('batch add', () => {
	afterEach(() => {
		cleanup();
		vi.unstubAllGlobals();
	});

	it('submits the parsed rows in paste order and clears the box when they all land', async () => {
		const stub = stubProxies({ pool: [] });
		render(ProxyPoolsPage);
		await openBatch('http://a.example.com:8080\nsocks5://operator:hunter2@b.example.com:1080\n');

		await fireEvent.click(screen.getByRole('button', { name: 'Add 2 proxies' }));

		await waitFor(() => expect(stub.creates.length).toBe(2));
		expect(stub.creates.map((body) => body.host)).toEqual(['a.example.com', 'b.example.com']);
		expect(stub.creates[1]).toEqual({
			label: 'b.example.com:1080',
			protocol: 'socks5',
			host: 'b.example.com',
			port: 1080,
			username: 'operator',
			password: 'hunter2',
			enabled: true
		});

		expect(await screen.findByText('Added 2 of 2.')).toBeTruthy();
		expect(value(screen.getByLabelText('Proxy URLs, one per line'))).toBe('');
	});

	it('keeps only the refused lines, with the reason, so a second run cannot duplicate', async () => {
		const stub = stubProxies({ pool: [], refuseHosts: ['b.example.com'] });
		render(ProxyPoolsPage);
		await openBatch('http://a.example.com:8080\nhttp://b.example.com:8080\n');

		await fireEvent.click(screen.getByRole('button', { name: 'Add 2 proxies' }));

		await waitFor(() => expect(stub.creates.length).toBe(2));
		expect(await screen.findByText(/Added 1 of 2\./)).toBeTruthy();

		// The box now holds the one line that did not make it, and the row says why.
		expect(value(screen.getByLabelText('Proxy URLs, one per line'))).toBe(
			'http://b.example.com:8080'
		);
		expect(screen.getByText('That address is already in the pool.')).toBeTruthy();
		expect(screen.getByRole('button', { name: 'Add 1 proxy' })).toBeTruthy();
	});

	it('forgets a previous run when the paste is edited, so no stale reason is shown', async () => {
		const stub = stubProxies({ pool: [], refuseHosts: ['b.example.com'] });
		render(ProxyPoolsPage);
		await openBatch('http://b.example.com:8080\n');

		await fireEvent.click(screen.getByRole('button', { name: 'Add 1 proxy' }));
		await waitFor(() => expect(stub.creates.length).toBe(1));
		expect(await screen.findByText('That address is already in the pool.')).toBeTruthy();

		// Retyping the paste must not keep the last run's refusal beside a row nobody has sent.
		await fireEvent.input(screen.getByLabelText('Proxy URLs, one per line'), {
			target: { value: 'http://c.example.com:8080\n' }
		});

		expect(screen.queryByText('That address is already in the pool.')).toBeNull();
		expect(screen.queryByText(/Added 0 of 1/)).toBeNull();
	});

	it('reports a rejected line and does not send it', async () => {
		const stub = stubProxies({ pool: [] });
		render(ProxyPoolsPage);
		await openBatch('http://a.example.com:8080\nsocks5://b.example.com\n');

		await fireEvent.click(screen.getByRole('button', { name: 'Add 1 proxy' }));

		await waitFor(() => expect(stub.creates.length).toBe(1));
		expect(stub.creates[0].host).toBe('a.example.com');
	});
});

describe('outbound settings', () => {
	afterEach(() => {
		cleanup();
		vi.unstubAllGlobals();
	});

	it('sends only the network group, so another settings tab cannot be overwritten', async () => {
		const stub = stubProxies();
		render(ProxyPoolsPage);
		await screen.findByLabelText('Last-resort proxy URL');

		await fireEvent.input(screen.getByLabelText('Last-resort proxy URL'), {
			target: { value: 'http://proxy.internal:8080' }
		});
		await fireEvent.click(screen.getByRole('button', { name: 'Save outbound settings' }));

		await waitFor(() => expect(stub.settingsPatches.length).toBe(1));
		expect(Object.keys(stub.settingsPatches[0])).toEqual(['network']);
		expect(stub.settingsPatches[0].network).toEqual({
			outbound_proxy_enabled: false,
			outbound_proxy_url: 'http://proxy.internal:8080',
			outbound_no_proxy: '',
			outbound_proxy_strategy: 'fallback'
		});
	});

	it('saves proxying on with no URL, because the pool is the route (D1)', async () => {
		// The pool engine made pool-only routing a real state: the walk serves the call and the URL is
		// the last resort after it. The form used to refuse this; now it stores it.
		const stub = stubProxies();
		render(ProxyPoolsPage);
		await screen.findByLabelText('Last-resort proxy URL');

		await fireEvent.click(
			screen.getByRole('checkbox', { name: /Route upstream calls through the proxy pool/ })
		);
		await fireEvent.click(screen.getByRole('button', { name: 'Save outbound settings' }));

		await waitFor(() => expect(stub.settingsPatches.length).toBe(1));
		expect(stub.settingsPatches[0]).toEqual({
			network: {
				outbound_proxy_enabled: true,
				outbound_proxy_url: '',
				outbound_no_proxy: '',
				outbound_proxy_strategy: 'fallback'
			}
		});
	});

	it('states a stored document that has proxying on with no URL, so the pool is the whole route', async () => {
		// A pool-only deployment is valid (D1), so the card states what will happen rather than
		// alarming: the pool carries traffic, and an empty pool dials direct.
		stubProxies({
			settings: settingsDocument({
				network: {
					outbound_proxy_enabled: true,
					outbound_proxy_url: '',
					outbound_no_proxy: '',
					outbound_proxy_strategy: 'round_robin'
				}
			})
		});
		render(ProxyPoolsPage);

		expect(await screen.findByText(/the pool above is the whole route/)).toBeTruthy();
		expect(screen.getByText(/upstream calls go direct until you add one/)).toBeTruthy();
	});

	it('shows the server message when a save is refused and keeps the draft', async () => {
		stubProxies({ writeStatus: 400 });
		render(ProxyPoolsPage);
		await screen.findByLabelText('Last-resort proxy URL');

		await fireEvent.input(screen.getByLabelText('Last-resort proxy URL'), {
			target: { value: 'http://proxy.internal:8080' }
		});
		await fireEvent.click(screen.getByRole('button', { name: 'Save outbound settings' }));

		expect(text(await screen.findByRole('alert'))).toContain('The gateway refused this value.');
		expect(value(screen.getByLabelText('Last-resort proxy URL'))).toBe(
			'http://proxy.internal:8080'
		);
	});
});
