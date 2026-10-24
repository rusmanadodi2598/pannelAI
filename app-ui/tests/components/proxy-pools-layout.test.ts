// Proxy Pools layout and field wiring (docs/SPEC-UI/001-SPEC-UI.md §6.9, §8.7).
//
// Two defects these hold shut, both measured in a browser on 2026-10-23. Nine columns left the Actions
// cell too little room to hold four 44px controls, so they wrapped into a stack and every row stood
// 105px tall; the Enabled cell had the same squeeze, and the mobile card layout put the word under the
// row label rather than beside the glyph. The second is the form's own: two hints existed on the page
// without being wired to their field, so the rule they state was never read with it.

import { cleanup, fireEvent, render, screen } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import ProxyPoolsPage from '../../src/routes/proxy-pools/+page.svelte';
import { proxyRow, stubProxies } from '../support/proxy-stub';

afterEach(() => {
	cleanup();
	vi.unstubAllGlobals();
});

async function firstRow(): Promise<HTMLElement> {
	stubProxies({ pool: [proxyRow()] });
	render(ProxyPoolsPage);
	const table = await screen.findByRole('table');
	return table.querySelector('tbody tr') as HTMLElement;
}

describe('the pool table at desktop width', () => {
	it('keeps the four row actions on one line instead of stacking them', async () => {
		const row = await firstRow();
		const group = row.querySelector('td:last-child > div') as HTMLElement;

		// The wrap is what fits a phone; `md:flex-nowrap` is what stops a 1440 table from
		// spending three rows' height on a column the browser squeezed to 141px.
		expect(group.className).toContain('flex-wrap');
		expect(group.className).toContain('md:flex-nowrap');
		expect(group.className).toContain('items-center');
	});

	it('holds the enabled glyph and its word in one cell, beside each other', async () => {
		const row = await firstRow();
		const cell = row.querySelector('td[data-label="Enabled"]') as HTMLElement;
		const line = cell.firstElementChild as HTMLElement;

		// Two siblings would each become their own grid item once the card layout takes over.
		expect(cell.children).toHaveLength(1);
		expect(line.className).toContain('items-center');
		expect(line.querySelector('button')).toBeTruthy();
		expect(line.textContent).toContain('Enabled');
	});
});

describe('the proxy form field wiring', () => {
	it('reads each hint with the field it belongs to', async () => {
		await firstRow();
		await fireEvent.click(screen.getByRole('button', { name: 'Edit Frankfurt egress' }));

		for (const [field, help] of [
			['Label', 'proxy-label-help'],
			['Host', 'proxy-host-help'],
			['Port', 'proxy-port-help'],
			['Username', 'proxy-username-help'],
			['Password', 'proxy-password-help']
		]) {
			const input = screen.getByLabelText(field);
			expect(input.getAttribute('aria-describedby'), `${field} has no hint wired`).toBe(help);
			expect(document.getElementById(help)?.textContent).toBeTruthy();
		}
	});
});
