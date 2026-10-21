// The proxy and credential tables' selection (docs/SPEC-UI/001-SPEC-UI.md §6.2, §8.11.9).
//
// Both tables put the row checkbox in the same cell as the row's own actions, and the proxy table's Enabled
// cell is an icon toggle that names its row rather than a text cell that reads as a label. The credential
// table has no Select-All of its own: the bar's, scoped to one endpoint, is that control.

import { cleanup, fireEvent, render, screen } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import ProxyTable from '$lib/components/ProxyTable.svelte';
import EndpointKeysTable from '$lib/components/EndpointKeysTable.svelte';
import { expectIconOnly } from '../support/icon-only';
import { endpointKey, proxy } from '../support/selection-fixtures';

afterEach(() => cleanup());

describe('the proxy table selection', () => {
	it('promotes Enabled from a text cell to an icon toggle that names its row', async () => {
		const ontoggle = vi.fn();
		render(ProxyTable, {
			proxies: [proxy({ id: 'prx_1', label: 'pool-a', enabled: true })],
			testingId: null,
			onedit: vi.fn(),
			ontest: vi.fn(),
			ondelete: vi.fn(),
			ontoggle
		});

		const name = 'Disable pool-a';
		const button = screen.getByRole('button', { name });
		expectIconOnly(button, name);
		await fireEvent.click(button);
		expect(ontoggle).toHaveBeenCalledWith(expect.objectContaining({ id: 'prx_1' }), false);
		// The word stays beside the glyph, so the state reads without depending on hue. The header
		// carries the same word, so the assertion is scoped to the cell's own text.
		expect(screen.getByText('Enabled', { selector: 'td span' })).toBeTruthy();
	});

	it('puts the row checkbox in the same cell as Edit, Test and Delete', async () => {
		const onselect = vi.fn();
		render(ProxyTable, {
			proxies: [proxy({ id: 'prx_1', label: 'pool-a' }), proxy({ id: 'prx_2', label: 'pool-b' })],
			testingId: null,
			onedit: vi.fn(),
			ontest: vi.fn(),
			ondelete: vi.fn(),
			ontoggle: vi.fn(),
			selected: [],
			onselect
		});

		const box = screen.getByRole('checkbox', { name: 'Select pool-b for bulk actions' });
		const edit = screen.getByRole('button', { name: 'Edit pool-b' });
		expect(
			box.closest('div')?.contains(edit),
			'checkbox and the action icons are not one group'
		).toBe(true);

		await fireEvent.click(box);
		expect(onselect).toHaveBeenCalledWith('prx_2', true);
	});
});

describe('the credential table inside one endpoint drawer', () => {
	it('renders a checkbox per credential beside its own actions', () => {
		render(EndpointKeysTable, {
			keys: [
				endpointKey({ id: 'uky_1', label: 'primary' }),
				endpointKey({ id: 'uky_2', label: 'spare', tags: ['event-week'] })
			],
			authType: 'api_key',
			testing: null,
			now: Date.now(),
			ontest: vi.fn(),
			onsettled: vi.fn(),
			onremove: vi.fn(),
			selected: [],
			onselect: vi.fn()
		});

		// The credential drawer selects per row; its Select-All is the bar's, scoped to this endpoint.
		expect(screen.getByRole('checkbox', { name: 'Select primary for bulk actions' })).toBeTruthy();
		// The label rides in the chip set the drawer renders, not in a ninth column.
		expect(screen.getByText('event-week')).toBeTruthy();
	});
});
