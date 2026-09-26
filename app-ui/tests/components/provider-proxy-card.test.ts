// The provider screen's proxy binding card (docs/PORT/009-PORT-PROVIDER-PROXY.md D1-D5, D9;
// docs/SPEC-UI/001-SPEC-UI.md §6.3, docs/SPEC-API/001-SPEC-API.md §7.14).
//
// The card is the reference's per-provider control (`NoAuthProxyCard.js`) in our settings-map shape:
// one provider's entry in `network.provider_proxies`, written whole-map on every change, and deleted
// when both selects are back on the default. The assertions are on the PATCH body, because that is
// where the rule lives: a panel that sent only its own provider's entry would erase every other
// binding, and the map is one settings value.
//
// What the copy must not misstate is the owner's pin rule (D4): a pin is the FIRST candidate and the
// other usable pools follow the strategy, so the card says that rather than repeating the reference's
// "pool selector is ignored when rotation is active". The reference disables the pool select once a
// rotation strategy is on; ours keeps both editable, and the sentence under them states what the
// engine will actually do.
//
// `None` is the reference's own sentinel (`__none__`, "None (direct)"), and it is a real mode here:
// the provider dials direct even while the global switch is on (D5).

import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import ProviderProxyCard from '$lib/components/ProviderProxyCard.svelte';
import { settingsDocument, networkGroup } from '../support/settings-document';
import { proxyRow, stubProxies, type ProxyStub } from '../support/proxy-stub';
import { PROXY_POOL_NONE } from '$lib/schemas/settings';

const PROVIDER = 'openai';
const POOL = 'prx_01HZZ9K2';
const POOL_LABEL = 'Frankfurt egress';

let stub: ProxyStub;

afterEach(() => {
	cleanup();
	vi.unstubAllGlobals();
});

function openWith(
	network: Record<string, unknown> = {},
	options: { pool?: Record<string, unknown>[] } = {}
): void {
	stub = stubProxies({
		pool: options.pool ?? [proxyRow()],
		settings: settingsDocument({ network: networkGroup(network) })
	});
	render(ProviderProxyCard, { props: { providerId: PROVIDER } });
}

/** The two selects, found by their own labels so no other control can satisfy the query. */
function poolSelect(): HTMLSelectElement {
	return screen.getByLabelText('Proxy pool') as HTMLSelectElement;
}

function strategySelect(): HTMLSelectElement {
	return screen.getByLabelText('Pool strategy') as HTMLSelectElement;
}

/** Waits for both reads to land, which is what enables the controls. */
async function ready(): Promise<void> {
	await waitFor(() => expect(poolSelect().disabled).toBe(false));
}

/** The routing group's sibling: a stored binding for this provider, or another one. */
function binding(overrides: Record<string, unknown> = {}): Record<string, unknown> {
	return {
		provider_proxies: { [PROVIDER]: { pool_id: POOL, strategy: 'fallback' } },
		...overrides
	};
}

describe('the provider proxy card', () => {
	it('reports an inheriting provider as Global and names the global setting', async () => {
		openWith({ outbound_proxy_enabled: false });
		await ready();

		expect(poolSelect().value).toBe('');
		expect(strategySelect().value).toBe('');
		expect(screen.getByText('Follows the global proxy setting (off).')).toBeTruthy();
	});

	it('names the global strategy it would walk when the global setting is on', async () => {
		openWith({ outbound_proxy_enabled: true, outbound_proxy_strategy: 'round_robin' });
		await ready();

		expect(screen.getByText('Follows the global proxy setting (on, Round robin).')).toBeTruthy();
	});

	it('shows a stored pin and states that it leads the walk', async () => {
		openWith({ provider_proxies: { [PROVIDER]: { pool_id: POOL, strategy: 'round_robin' } } });
		await ready();

		expect(poolSelect().value).toBe(POOL);
		expect(strategySelect().value).toBe('round_robin');
		expect(
			screen.getByText(
				`Pinned to ${POOL_LABEL}, which leads every attempt; the other usable pools follow with Round robin.`
			)
		).toBeTruthy();
	});

	it('writes a pin by sending the whole map, keeping every other provider binding', async () => {
		openWith({ provider_proxies: { anthropic: { pool_id: 'prx_other' } } });
		await ready();

		await fireEvent.change(poolSelect(), { target: { value: POOL } });

		await waitFor(() => expect(stub.settingsPatches.length).toBe(1));
		expect(stub.settingsPatches[0]).toEqual({
			network: {
				provider_proxies: {
					anthropic: { pool_id: 'prx_other' },
					[PROVIDER]: { pool_id: POOL }
				}
			}
		});
		await waitFor(() => expect(poolSelect().value).toBe(POOL));
		expect(screen.getByText(`This provider now uses ${POOL_LABEL} first.`)).toBeTruthy();
	});

	it('merges onto a fresh read, so a binding another screen stored since the load survives', async () => {
		openWith();
		await ready();

		// Another screen pins its own provider between this page's load and this write.
		const network = stub.settings.network as Record<string, unknown>;
		network.provider_proxies = { openrouter: { pool_id: 'prx_later', strategy: 'fallback' } };

		await fireEvent.change(poolSelect(), { target: { value: PROXY_POOL_NONE } });

		await waitFor(() => expect(stub.settingsPatches.length).toBe(1));
		expect(stub.settingsPatches[0]).toEqual({
			network: {
				provider_proxies: {
					openrouter: { pool_id: 'prx_later', strategy: 'fallback' },
					[PROVIDER]: { pool_id: PROXY_POOL_NONE }
				}
			}
		});
	});

	it('writes the none sentinel, which dials direct past the global setting', async () => {
		openWith({ outbound_proxy_enabled: true });
		await ready();

		await fireEvent.change(poolSelect(), { target: { value: PROXY_POOL_NONE } });

		await waitFor(() => expect(stub.settingsPatches.length).toBe(1));
		expect(stub.settingsPatches[0]).toEqual({
			network: { provider_proxies: { [PROVIDER]: { pool_id: PROXY_POOL_NONE } } }
		});
		expect(
			await screen.findByText(
				'Dials direct; the global proxy setting is not used for this provider.'
			)
		).toBeTruthy();
	});

	it('writes a strategy override while the pool stays global', async () => {
		openWith({ outbound_proxy_enabled: true, outbound_proxy_strategy: 'fallback' });
		await ready();

		await fireEvent.change(strategySelect(), { target: { value: 'round_robin' } });

		await waitFor(() => expect(stub.settingsPatches.length).toBe(1));
		expect(stub.settingsPatches[0]).toEqual({
			network: { provider_proxies: { [PROVIDER]: { strategy: 'round_robin' } } }
		});
		expect(screen.getByText('Pool strategy saved.')).toBeTruthy();
		expect(
			screen.getByText(
				'Follows the global pool with its own strategy (Round robin), and proxying is on.'
			)
		).toBeTruthy();
	});

	it('deletes the entry when the pool returns to Global and no strategy is left', async () => {
		openWith(binding());
		await ready();

		// Back to Global first: the entry keeps its strategy, because that is still a real override.
		await fireEvent.change(poolSelect(), { target: { value: '' } });
		await waitFor(() => expect(stub.settingsPatches.length).toBe(1));
		expect(stub.settingsPatches[0]).toEqual({
			network: { provider_proxies: { [PROVIDER]: { strategy: 'fallback' } } }
		});

		// Then the strategy follows the global one, which leaves nothing to store: the entry goes.
		await fireEvent.change(strategySelect(), { target: { value: '' } });
		await waitFor(() => expect(stub.settingsPatches.length).toBe(2));
		expect(stub.settingsPatches[1]).toEqual({ network: { provider_proxies: {} } });
		expect(screen.getByText('Back on the global proxy setting.')).toBeTruthy();
	});

	it('names a pinned pool that is no longer in the list rather than reading it as Global', async () => {
		// A pinned row can be deleted from the pool screen after the pin was stored (the delete route
		// does not know about bindings), and a select that silently fell back to Global would drop the
		// stored pin on the next strategy change.
		openWith({ provider_proxies: { [PROVIDER]: { pool_id: 'prx_gone' } } }, { pool: [proxyRow()] });
		await ready();

		expect(poolSelect().value).toBe('prx_gone');
		expect(screen.getByRole('option', { name: 'prx_gone (missing)' })).toBeTruthy();
		expect(
			screen.getByText(
				'Pinned to prx_gone, which is no longer in the pool; the remaining pools carry this provider with Fallback.'
			)
		).toBeTruthy();

		await fireEvent.change(strategySelect(), { target: { value: 'round_robin' } });

		await waitFor(() => expect(stub.settingsPatches.length).toBe(1));
		expect(stub.settingsPatches[0]).toEqual({
			network: {
				provider_proxies: { [PROVIDER]: { pool_id: 'prx_gone', strategy: 'round_robin' } }
			}
		});
	});

	it('reports the gateway refusal and puts the selects back', async () => {
		stub = stubProxies({
			pool: [proxyRow()],
			settings: settingsDocument({ network: networkGroup() }),
			writeStatus: 400
		});
		render(ProviderProxyCard, { props: { providerId: PROVIDER } });
		await ready();

		await fireEvent.change(poolSelect(), { target: { value: POOL } });

		expect(await screen.findByText('The gateway refused this value.')).toBeTruthy();
		expect(poolSelect().value).toBe('');
		expect(screen.getByText('Follows the global proxy setting (off).')).toBeTruthy();
	});

	it('reports a pool list that could not be read without losing the stored binding', async () => {
		stub = stubProxies({
			settings: settingsDocument({
				network: networkGroup({ provider_proxies: { [PROVIDER]: { pool_id: POOL } } })
			}),
			readStatus: 500
		});
		render(ProviderProxyCard, { props: { providerId: PROVIDER } });
		await ready();

		expect(
			await screen.findByText('The pool list could not be read: The pool store is unreachable.')
		).toBeTruthy();
		// The binding is still what the settings document says, and it can still be changed.
		expect(poolSelect().value).toBe(POOL);
	});
});
