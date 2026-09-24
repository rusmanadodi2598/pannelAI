// Row actions render as icon-only buttons (owner directive, 2026-09-24).
//
// The two tables of the Endpoint & Key page are asserted here together because the contract is one
// contract: when the visible text goes, the button must still carry the action's name as its accessible
// name, it must say so on hover, and the glyph must be decorative so a screen reader does not read the
// icon and the name as two things. A button that kept its glyph and lost its name would still look right
// and be unusable, which is the failure this file exists to catch.

import { cleanup, fireEvent, render, screen, within } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import EndpointKeysTable from '../../src/lib/components/EndpointKeysTable.svelte';
import GatewayKeysTab from '../../src/lib/components/GatewayKeysTab.svelte';
import { endpointKeyRow } from '../support/endpoint-stub';
import { stubGatewayKeys } from '../support/gateway-key-stub';
import type { EndpointKey } from '$lib/schemas/endpoint';

afterEach(() => {
	cleanup();
	vi.unstubAllGlobals();
});

/** Asserts the icon-only contract on one button. */
function expectIconOnly(button: HTMLElement, name: string): void {
	expect(button.textContent?.trim(), `${name} still renders text`).toBe('');
	expect(button.getAttribute('title'), `${name} has no tooltip`).toBe(name);

	const glyph = button.querySelector('svg');
	expect(glyph, `${name} has no glyph`).toBeTruthy();
	expect(glyph?.getAttribute('aria-hidden'), `${name} glyph is not decorative`).toBe('true');
}

async function gatewayRow(): Promise<HTMLElement> {
	stubGatewayKeys();
	render(GatewayKeysTab);
	return (await screen.findByText('Laptop')).closest('tr') as HTMLElement;
}

describe('gateway key row actions', () => {
	it('renders rename, disable, and revoke as named icons', async () => {
		const row = await gatewayRow();

		for (const name of ['Rename', 'Disable', 'Revoke']) {
			expectIconOnly(within(row).getByRole('button', { name }), name);
		}
	});

	it('renders the edit state the same way, so the cell does not mix two idioms', async () => {
		const row = await gatewayRow();

		await fireEvent.click(within(row).getByRole('button', { name: 'Rename' }));

		for (const name of ['Save', 'Cancel']) {
			expectIconOnly(within(row).getByRole('button', { name }), name);
		}
	});
});

describe('endpoint key row actions', () => {
	it('renders test, disable, and delete as named icons', () => {
		const keys = [endpointKeyRow()] as unknown as EndpointKey[];
		render(EndpointKeysTable, {
			props: {
				keys,
				authType: 'api_key',
				testing: null,
				now: Date.parse('2026-09-21T12:00:00Z'),
				ontest: () => {},
				onsettled: () => {},
				onremove: () => {}
			}
		});

		const row = screen.getByText('First').closest('tr') as HTMLElement;

		for (const name of ['Test', 'Disable', 'Delete']) {
			expectIconOnly(within(row).getByRole('button', { name }), name);
		}
	});
});
