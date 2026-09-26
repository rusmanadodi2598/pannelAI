// Settings screen tests (docs/SPEC-UI/001-SPEC-UI.md §6.13, §7.6, §8.4).
//
// The load-bearing rule is that a tab sends only its own group, because §6.13 says `PATCH` is per
// changed group: a tab that sent the whole document could overwrite a key another tab owns with a
// value it read before that tab changed it. The assertions are on the PATCH body the panel produced,
// so the rule is checked where it actually lives.
//
// The stub answers a read and a write differently on purpose: a test that makes both fail cannot tell
// "the load broke" from "the save broke".

import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import SettingsPage from '../../src/routes/settings/+page.svelte';
import { settingsDocument } from '../support/settings-document';

vi.mock('$app/paths', () => ({ resolve: (path: string) => path }));

type Stub = {
	patches: Record<string, unknown>[];
	settings: Record<string, unknown>;
	readStatus: number;
	writeStatus: number;
};

function stubSettings(overrides: Partial<Stub> = {}): Stub {
	const stub: Stub = {
		patches: [],
		settings: settingsDocument(),
		readStatus: 200,
		writeStatus: 200,
		...overrides
	};

	const envelope = (code: string, message: string): string =>
		JSON.stringify({ error: { code, message } });

	vi.stubGlobal('fetch', async (input: unknown, init?: RequestInit) => {
		const method = init?.method ?? 'GET';

		if (method === 'PATCH') {
			stub.patches.push(JSON.parse(String(init?.body ?? '{}')));

			if (stub.writeStatus !== 200) {
				return new Response(envelope('VALIDATION_ERROR', 'The gateway refused this value.'), {
					status: stub.writeStatus,
					headers: { 'content-type': 'application/json' }
				});
			}

			return new Response(JSON.stringify(stub.settings), {
				status: 200,
				headers: { 'content-type': 'application/json' }
			});
		}

		if (stub.readStatus !== 200) {
			return new Response(envelope('INTERNAL_ERROR', 'The settings store is unreachable.'), {
				status: stub.readStatus,
				headers: { 'content-type': 'application/json' }
			});
		}

		return new Response(JSON.stringify(stub.settings), {
			status: 200,
			headers: { 'content-type': 'application/json' }
		});
	});

	return stub;
}

async function openTab(name: string): Promise<void> {
	await screen.findByRole('tablist');
	await fireEvent.click(screen.getByRole('tab', { name }));
	await screen.findByRole('tabpanel');
}

describe('SettingsPage', () => {
	afterEach(() => {
		cleanup();
		vi.unstubAllGlobals();
	});

	it('renders the five §6.13 tabs', async () => {
		stubSettings();
		render(SettingsPage);

		await screen.findByRole('tablist');

		for (const label of ['Security', 'Routing', 'Network', 'Logging', 'Token Saver']) {
			expect(screen.getByRole('tab', { name: label }), label).toBeTruthy();
		}
	});

	it('shows the loaded routing values on the Routing tab', async () => {
		stubSettings();
		render(SettingsPage);
		await openTab('Routing');

		expect((screen.getByLabelText('Default combo strategy') as HTMLSelectElement).value).toBe(
			'fallback'
		);
		expect((screen.getByLabelText('Combo sticky limit') as HTMLInputElement).value).toBe('1');
		expect((screen.getByLabelText('Routing sticky limit') as HTMLInputElement).value).toBe('3');
		expect((screen.getByLabelText('Credential rotation') as HTMLSelectElement).value).toBe(
			'fill-first'
		);
	});

	it('sends only the routing group when the Routing tab saves', async () => {
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

		expect(Object.keys(stub.patches[0])).toEqual(['routing']);
		expect(stub.patches[0].routing).toEqual({
			combo_strategy: 'fallback',
			combo_sticky_limit: 1,
			sticky_limit: 5,
			fallback_strategy: 'fill-first'
		});
	});

	it('refuses a routing limit below the API floor without sending anything', async () => {
		const stub = stubSettings();
		render(SettingsPage);
		await openTab('Routing');

		await fireEvent.input(screen.getByLabelText('Routing sticky limit'), {
			target: { value: '0' }
		});
		await fireEvent.click(screen.getByRole('button', { name: 'Save routing settings' }));

		// Asserted on the field's own validation message, not on a shared hint substring: every hint on
		// this tab ends with "At least 1", so a substring match would pass without the form refusing.
		expect(await screen.findByText('The routing sticky limit must be at least 1.')).toBeTruthy();
		expect(stub.patches.length).toBe(0);
	});

	it('sends only the logging group when the Logging tab saves', async () => {
		const stub = stubSettings();
		render(SettingsPage);
		await openTab('Logging');

		await fireEvent.input(screen.getByLabelText('Retention days'), { target: { value: '30' } });
		await fireEvent.click(screen.getByRole('button', { name: 'Save logging settings' }));

		await waitFor(() => {
			expect(stub.patches.length).toBe(1);
		});

		expect(Object.keys(stub.patches[0])).toEqual(['logging']);
		expect(stub.patches[0].logging).toEqual({
			request_capture_enabled: false,
			retention_days: 30,
			capture_body_max_bytes: 65536,
			observability_max_records: 1000
		});
	});

	it('states the privacy cost of turning capture on', async () => {
		stubSettings();
		render(SettingsPage);
		await openTab('Logging');

		// The warning names what is stored and the risk, per §6.13's "capture sets a privacy cost".
		expect(screen.getByText(/stores the full request and response body/i)).toBeTruthy();
		expect(screen.getByText(/can include sensitive content/i)).toBeTruthy();
	});

	it('links the Network tab to Proxy Pools instead of repeating the outbound form', async () => {
		// §6.13 gives this tab a link, not a second editor: one configuration with two editors is how
		// the two drift. The link is real because the route exists, which R-24 requires.
		const stub = stubSettings();
		render(SettingsPage);
		await openTab('Network');

		const link = screen.getByRole('link', { name: 'Open Proxy Pools' });
		expect(link.getAttribute('href')).toBe('/proxy-pools');
		expect(screen.queryByLabelText('Last-resort proxy URL')).toBeNull();

		// Nothing is written from this tab any more, so a click here must not produce a PATCH.
		await fireEvent.click(link);
		expect(stub.patches).toEqual([]);
	});

	it('links the Token Saver tab to its own screen instead of repeating the form', async () => {
		stubSettings();
		render(SettingsPage);
		await openTab('Token Saver');

		expect(screen.getByRole('link', { name: 'Open Token Saver' }).getAttribute('href')).toBe(
			'/token-saver'
		);
		expect(screen.queryByRole('button', { name: 'Save RTK' })).toBeNull();
	});

	it('shows the server message when a save is refused, and keeps the draft', async () => {
		const stub = stubSettings({ writeStatus: 400 });
		render(SettingsPage);
		await openTab('Routing');

		await fireEvent.input(screen.getByLabelText('Routing sticky limit'), {
			target: { value: '5' }
		});
		await fireEvent.click(screen.getByRole('button', { name: 'Save routing settings' }));

		expect(await screen.findByText('The gateway refused this value.')).toBeTruthy();
		expect(stub.patches.length).toBe(1);
	});

	it('reports a load failure and offers a retry', async () => {
		const stub = stubSettings({ readStatus: 500 });
		render(SettingsPage);

		expect(await screen.findByText('Settings could not be loaded')).toBeTruthy();

		const before = stub.patches.length;
		await fireEvent.click(screen.getByRole('button', { name: 'Try again' }));

		await waitFor(() => {
			expect(stub.patches.length).toBe(before);
		});
	});
});
