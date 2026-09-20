// Proxy Pools write tests (docs/SPEC-UI/001-SPEC-UI.md §6.9, §8.6.3).
//
// The rules a reader cannot check by looking at the screen: what the panel sends for a create and for
// an edit, that an empty password keeps a stored secret rather than clearing it, that the candidate
// test stores nothing, and that every write ends in a re-read so the table shows what the API stored
// rather than what the page guessed.

import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import ProxyPoolsPage from '../../src/routes/proxy-pools/+page.svelte';
import { squashed, text, value } from '../support/dom';
import { proxyRow, stubProxies } from '../support/proxy-stub';

describe('creating a proxy', () => {
	afterEach(() => {
		cleanup();
		vi.unstubAllGlobals();
	});

	it('creates a candidate and shows the row the API stored', async () => {
		const stub = stubProxies({ pool: [] });
		render(ProxyPoolsPage);
		await screen.findByText('No proxies yet');

		await fireEvent.click(screen.getByRole('button', { name: 'Add a proxy' }));
		await fireEvent.input(screen.getByLabelText('Label'), {
			target: { value: 'Frankfurt egress' }
		});
		await fireEvent.change(screen.getByLabelText('Protocol'), { target: { value: 'socks5' } });
		await fireEvent.input(screen.getByLabelText('Host'), {
			target: { value: 'Proxy.Example.COM' }
		});
		await fireEvent.input(screen.getByLabelText('Port'), { target: { value: '1080' } });
		await fireEvent.input(screen.getByLabelText('Username'), { target: { value: 'operator' } });
		await fireEvent.input(screen.getByLabelText('Password'), { target: { value: 'hunter2' } });
		await fireEvent.click(screen.getByRole('button', { name: 'Add proxy' }));

		await waitFor(() => expect(stub.creates.length).toBe(1));
		expect(stub.creates[0]).toEqual({
			label: 'Frankfurt egress',
			protocol: 'socks5',
			host: 'proxy.example.com',
			port: 1080,
			username: 'operator',
			password: 'hunter2',
			enabled: true
		});

		// The re-read is what puts the stored row on screen (§8.6.3).
		expect(await screen.findByText('Frankfurt egress')).toBeTruthy();
	});

	it('refuses a port the API would reject, without sending anything', async () => {
		const stub = stubProxies({ pool: [] });
		render(ProxyPoolsPage);
		await screen.findByText('No proxies yet');

		await fireEvent.click(screen.getByRole('button', { name: 'Add a proxy' }));
		await fireEvent.input(screen.getByLabelText('Label'), { target: { value: 'Bad port' } });
		await fireEvent.input(screen.getByLabelText('Host'), {
			target: { value: 'proxy.example.com' }
		});
		await fireEvent.input(screen.getByLabelText('Port'), { target: { value: '70000' } });
		await fireEvent.click(screen.getByRole('button', { name: 'Add proxy' }));

		expect(text(await screen.findByRole('alert'))).toContain('Ports end at 65535.');
		expect(stub.creates).toEqual([]);
	});

	it('tests an unsaved candidate before saving, and stores nothing', async () => {
		const stub = stubProxies({ pool: [] });
		render(ProxyPoolsPage);
		await screen.findByText('No proxies yet');

		await fireEvent.click(screen.getByRole('button', { name: 'Add a proxy' }));
		await fireEvent.input(screen.getByLabelText('Host'), {
			target: { value: 'proxy.example.com' }
		});
		await fireEvent.input(screen.getByLabelText('Port'), { target: { value: '8080' } });
		// No label is filled: a candidate can be probed before it has a name.
		await fireEvent.click(screen.getByRole('button', { name: 'Test this candidate' }));

		await waitFor(() => expect(stub.candidateTests.length).toBe(1));
		expect(stub.candidateTests[0]).toEqual({
			protocol: 'http',
			host: 'proxy.example.com',
			port: 8080,
			username: '',
			password: ''
		});
		expect(stub.creates).toEqual([]);
		expect(await screen.findByText(/Reachable in 42ms, checked/)).toBeTruthy();
	});
});

describe('editing a proxy', () => {
	afterEach(() => {
		cleanup();
		vi.unstubAllGlobals();
	});

	it('keeps the stored password when the field is left empty', async () => {
		const stub = stubProxies({ pool: [proxyRow({ has_password: true })] });
		render(ProxyPoolsPage);
		await screen.findByRole('table');

		await fireEvent.click(screen.getByRole('button', { name: 'Edit Frankfurt egress' }));

		// Never prefilled: the API does not return the value, so the panel cannot show it.
		expect(value(screen.getByLabelText('Password'))).toBe('');
		expect(screen.getByText(/Leave this empty to keep it/)).toBeTruthy();

		await fireEvent.input(screen.getByLabelText('Label'), {
			target: { value: 'Frankfurt egress two' }
		});
		await fireEvent.click(screen.getByRole('button', { name: 'Save proxy' }));

		await waitFor(() => expect(stub.patches.length).toBe(1));
		expect(stub.patches[0].id).toBe('prx_01HZZ9K2');
		expect(stub.patches[0].body.password).toBe('');

		// The fake API applies the same keep rule the real one does, so the row still reports a secret.
		expect(await screen.findByText('Frankfurt egress two')).toBeTruthy();
		expect(screen.getByText('Set')).toBeTruthy();
	});

	it('replaces the stored password when a new one is typed', async () => {
		const stub = stubProxies({ pool: [proxyRow({ has_password: false })] });
		render(ProxyPoolsPage);
		await screen.findByRole('table');
		expect(screen.getByText('Not set')).toBeTruthy();

		await fireEvent.click(screen.getByRole('button', { name: 'Edit Frankfurt egress' }));
		expect(screen.getByText('No password is stored.')).toBeTruthy();

		await fireEvent.input(screen.getByLabelText('Password'), { target: { value: 'hunter2' } });
		await fireEvent.click(screen.getByRole('button', { name: 'Save proxy' }));

		await waitFor(() => expect(stub.patches.length).toBe(1));
		expect(stub.patches[0].body.password).toBe('hunter2');
		expect(await screen.findByText('Set')).toBeTruthy();
	});

	it('shows a refused save as an alert and keeps the dialog open', async () => {
		const stub = stubProxies({ pool: [proxyRow()], writeStatus: 400 });
		render(ProxyPoolsPage);
		await screen.findByRole('table');

		await fireEvent.click(screen.getByRole('button', { name: 'Edit Frankfurt egress' }));
		await fireEvent.click(screen.getByRole('button', { name: 'Save proxy' }));

		expect(text(await screen.findByRole('alert'))).toContain('The gateway refused this value.');
		expect(screen.getByRole('button', { name: 'Save proxy' })).toBeTruthy();
		expect(stub.patches.length).toBe(1);
	});
});

describe('testing and deleting a stored proxy', () => {
	afterEach(() => {
		cleanup();
		vi.unstubAllGlobals();
	});

	it('tests a row, announces the result, and shows the stored status', async () => {
		const stub = stubProxies({ pool: [proxyRow()] });
		render(ProxyPoolsPage);
		await screen.findByRole('table');

		await fireEvent.click(screen.getByRole('button', { name: 'Test Frankfurt egress' }));

		await waitFor(() => expect(stub.testedIds).toEqual(['prx_01HZZ9K2']));
		expect(await screen.findByText(/Frankfurt egress: Reachable in 42ms/)).toBeTruthy();
		// The row carries the stored result once the re-read lands, which is a second async step, so it
		// is waited for rather than read straight after the notice.
		await waitFor(() => expect(text(screen.getByRole('table'))).toContain('Reachable in 42ms'));
	});

	it('announces a failed probe as a result with its reason, not as an error banner', async () => {
		stubProxies({
			pool: [proxyRow()],
			testState: 'fail',
			testMessage: 'the proxy rejected the credentials'
		});
		render(ProxyPoolsPage);
		await screen.findByRole('table');

		await fireEvent.click(screen.getByRole('button', { name: 'Test Frankfurt egress' }));

		expect(
			await screen.findByText(
				'Frankfurt egress: Failed in 5000ms. the proxy rejected the credentials'
			)
		).toBeTruthy();
		expect(screen.queryByRole('alert')).toBeNull();
	});

	it('names the object and the real consequence in the delete confirmation', async () => {
		const stub = stubProxies({ pool: [proxyRow()] });
		render(ProxyPoolsPage);
		await screen.findByRole('table');

		await fireEvent.click(screen.getByRole('button', { name: 'Delete Frankfurt egress' }));

		// §8.5 wants the object named, and the consequence stated precisely: the outbound path is a
		// separate setting, so deleting a row does not reroute anything.
		const dialog = squashed(screen.getByRole('dialog'));
		expect(dialog).toContain('Frankfurt egress');
		expect(dialog).toContain('proxy.example.com:8443');
		expect(dialog).toContain('does not change the path upstream calls take');

		await fireEvent.click(screen.getByRole('button', { name: 'Delete the proxy' }));

		await waitFor(() => expect(stub.deletes).toEqual(['prx_01HZZ9K2']));
		expect(await screen.findByText('No proxies yet')).toBeTruthy();
	});
});
