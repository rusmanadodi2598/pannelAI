// Proxy Pools display tests (docs/SPEC-UI/001-SPEC-UI.md §6.9).
//
// What the screen shows and what it refuses to imply. Three of these assert an absence, which is the
// harder half: a password value must not reach the page, a never-tested row must not read as a
// failure, and a prober state the panel does not know must not be swallowed.

import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import ProxyPoolsPage from '../../src/routes/proxy-pools/+page.svelte';
import { formatTimestamp } from '$lib/utils/time';
import { value, text } from '../support/dom';
import { proxyRow, stubProxies } from '../support/proxy-stub';
import { settingsDocument, networkGroup } from '../support/settings-document';

async function loaded(): Promise<void> {
	await screen.findByRole('heading', { name: 'Proxy Pools' });
}

describe('ProxyPoolsPage', () => {
	afterEach(() => {
		cleanup();
		vi.unstubAllGlobals();
	});

	it('says what it is loading', async () => {
		stubProxies();
		render(ProxyPoolsPage);
		await loaded();

		expect(screen.getByText('Loading the proxy pool')).toBeTruthy();
		await screen.findByText('No proxies yet');
	});

	it('shows the empty state with the reason and the action that fills it', async () => {
		stubProxies({ pool: [] });
		render(ProxyPoolsPage);

		expect(await screen.findByText('No proxies yet')).toBeTruthy();
		expect(screen.getByText(/route upstream calls through it once proxying is on/)).toBeTruthy();
		expect(screen.getAllByRole('button', { name: 'Add a proxy' }).length).toBeGreaterThan(0);
	});

	it('offers a retry when the pool could not be read', async () => {
		stubProxies({ readStatus: 500 });
		render(ProxyPoolsPage);

		expect(await screen.findByText('The proxy pool could not be loaded')).toBeTruthy();
		expect(screen.getByText('The pool store is unreachable.')).toBeTruthy();
		expect(screen.getByRole('button', { name: 'Try again' })).toBeTruthy();
	});

	it('renders the columns §6.9 asks for', async () => {
		stubProxies({ pool: [proxyRow()] });
		render(ProxyPoolsPage);
		await screen.findByRole('table');

		for (const column of [
			'Label',
			'Protocol',
			'Host',
			'Port',
			'Username',
			'Password',
			'Enabled',
			'Last test',
			'Actions'
		]) {
			expect(screen.getByRole('columnheader', { name: column }), column).toBeTruthy();
		}
	});

	it('reports a stored password as set without putting the value on the page', async () => {
		stubProxies({ pool: [proxyRow({ has_password: true })] });
		render(ProxyPoolsPage);
		await screen.findByRole('table');

		expect(screen.getByText('Set')).toBeTruthy();
		expect(document.body.textContent).not.toContain('hunter2');
	});

	it('says a row was never tested rather than reading it as a failure', async () => {
		stubProxies({ pool: [proxyRow()] });
		render(ProxyPoolsPage);

		expect(await screen.findByText('Not tested')).toBeTruthy();
	});

	it('shows the stored state, latency, and checked time', async () => {
		stubProxies({
			pool: [
				proxyRow({
					status: { state: 'ok', latency_ms: 42, checked_at: '2026-09-19T09:05:00Z' }
				})
			]
		});
		render(ProxyPoolsPage);
		await screen.findByText(/Reachable in 42ms/);

		// The expectation is built with the panel's own formatter, because the rendering is
		// locale-dependent and a literal date would assert the test runner's locale instead of the
		// screen's behaviour.
		expect(text(screen.getByRole('table'))).toContain(formatTimestamp('2026-09-19T09:05:00Z'));
	});

	it('shows a failed probe with the reason the gateway reported', async () => {
		stubProxies({
			pool: [
				proxyRow({
					status: {
						state: 'fail',
						latency_ms: 5000,
						checked_at: '2026-09-19T09:05:00Z',
						message: 'the proxy rejected the credentials'
					}
				})
			]
		});
		render(ProxyPoolsPage);

		expect(await screen.findByText(/Failed in 5000ms/)).toBeTruthy();
		expect(screen.getByText('the proxy rejected the credentials')).toBeTruthy();
	});

	it('renders a prober state the panel does not know as it arrived', async () => {
		stubProxies({
			pool: [
				proxyRow({
					status: { state: 'degraded', latency_ms: 900, checked_at: '2026-09-19T09:05:00Z' }
				})
			]
		});
		render(ProxyPoolsPage);

		expect(await screen.findByText(/degraded in 900ms/)).toBeTruthy();
	});

	it('previews a paste and lists a rejected line with its number and reason', async () => {
		stubProxies();
		render(ProxyPoolsPage);
		await screen.findByText('No proxies yet');

		await fireEvent.click(screen.getByRole('button', { name: 'Add several at once' }));
		await fireEvent.input(screen.getByLabelText('Proxy URLs, one per line'), {
			target: { value: 'http://a.example.com:8080\nnot-a-url\nsocks5://b.example.com:1080\n' }
		});

		expect(await screen.findByText('2 ready, 1 rejected.')).toBeTruthy();
		expect(
			screen.getByText('Line 2: Start the line with http://, https://, or socks5://.')
		).toBeTruthy();
		expect(screen.getByRole('button', { name: 'Add 2 proxies' })).toBeTruthy();
	});

	it('re-reads the pool when the operator asks for it', async () => {
		const stub = stubProxies({ pool: [proxyRow()] });
		render(ProxyPoolsPage);
		await screen.findByRole('table');

		const poolReads = (): number =>
			stub.reads.filter((url) => url.split('?')[0].endsWith('/proxies')).length;
		const before = poolReads();

		await fireEvent.click(screen.getByRole('button', { name: 'Refresh now' }));

		// §8.6.2: the control repeats the pool read, so a row another operator just added shows up without a
		// page reload.
		await waitFor(() => expect(poolReads()).toBeGreaterThan(before));
	});

	it('shows whether a pasted line carried a password, not the password', async () => {
		stubProxies();
		render(ProxyPoolsPage);
		await screen.findByText('No proxies yet');

		await fireEvent.click(screen.getByRole('button', { name: 'Add several at once' }));
		await fireEvent.input(screen.getByLabelText('Proxy URLs, one per line'), {
			target: { value: 'socks5://operator:hunter2@b.example.com:1080\n' }
		});

		const preview = await screen.findByRole('table');
		expect(text(preview)).toContain('Set');
		expect(text(preview)).not.toContain('hunter2');
	});
});

describe('the outbound card', () => {
	afterEach(() => {
		cleanup();
		vi.unstubAllGlobals();
	});

	it('states that the settings are global and that per-endpoint binding is deferred', async () => {
		// The two claims §6.9 requires. Without them an operator reads the pool as a decoration or hunts
		// for a per-endpoint control that SPEC-API §7.11 does not implement.
		stubProxies();
		render(ProxyPoolsPage);

		expect(await screen.findByRole('heading', { name: 'Outbound proxy' })).toBeTruthy();
		expect(
			screen.getByText(/Global, and the setting the gateway actually routes with/)
		).toBeTruthy();
		expect(screen.getByText(/URL field is the last resort/)).toBeTruthy();
		// The card names where the per-provider control lives (docs/PORT/009-PORT-PROVIDER-PROXY.md D9)
		// and what is still deferred, so neither is left for a reader to hunt for.
		expect(
			screen.getByText(/unless a provider has its own binding on its provider screen/)
		).toBeTruthy();
		expect(screen.getByText(/Per-endpoint binding is still deferred/)).toBeTruthy();
	});

	it('shows the stored outbound values, strategy included', async () => {
		stubProxies({
			settings: settingsDocument({
				network: networkGroup({
					outbound_proxy_enabled: true,
					outbound_proxy_url: 'http://proxy.internal:8080',
					outbound_no_proxy: 'localhost',
					outbound_proxy_strategy: 'round_robin'
				})
			})
		});
		render(ProxyPoolsPage);

		// The label the component carries at HEAD ('Last-resort proxy URL') and the strategy the test's
		// own name promises: HEAD shipped the rename in the component without the matching query here,
		// so this pass adopts the one-line repair rather than committing a red test.
		expect(value(await screen.findByLabelText('Last-resort proxy URL'))).toBe(
			'http://proxy.internal:8080'
		);
		expect(value(screen.getByLabelText('Bypass the proxy for these hosts'))).toBe('localhost');
		expect(value(screen.getByLabelText('Pool strategy'))).toBe('round_robin');
	});

	it('says so when the settings document cannot be read, without hiding the pool', async () => {
		// The card loads its own document, so a settings failure is stated where it happened and the
		// pool above it still renders.
		stubProxies({ pool: [proxyRow()], settingsStatus: 500 });
		render(ProxyPoolsPage);

		expect(await screen.findByText('The outbound proxy settings could not be loaded')).toBeTruthy();
		expect(screen.getByRole('table')).toBeTruthy();
	});
});
