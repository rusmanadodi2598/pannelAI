// Proxy Pools control glyph tests (owner directive, 2026-09-26, PORT 007).
//
// Every control the directive names carries a glyph from the icon maps, never an emoticon and never
// bare text, and the screen carries exactly one refresh control: the shared "Refresh now" control is
// the only one, so the toolbar's plain "Refresh" button is asserted as an absence. The row actions
// follow the icon-only contract the Endpoint & Key tables already hold, with the name carrying the
// row it acts on, and the destructive action keeps the one-text-colour rule measured on 2026-09-24.

import { cleanup, render, screen, within } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import ProxyPoolsPage from '../../src/routes/proxy-pools/+page.svelte';
import { expectIconOnly } from '../support/icon-only';
import { proxyRow, stubProxies } from '../support/proxy-stub';

/** Asserts glyph + label on a control that keeps its visible text (R-04, R-31). */
function expectGlyphAndLabel(button: HTMLElement, label: string): void {
	const glyph = button.querySelector('svg');
	expect(glyph, `${label} has no glyph`).toBeTruthy();
	expect(glyph?.getAttribute('aria-hidden'), `${label} glyph is not decorative`).toBe('true');
	expect(button.textContent?.trim(), `${label} lost its visible label`).toContain(label);
}

describe('proxy pool control glyphs', () => {
	afterEach(() => {
		cleanup();
		vi.unstubAllGlobals();
	});

	it('carries the add glyph on the toolbar "Add a proxy" control', async () => {
		stubProxies({ pool: [proxyRow()] });
		render(ProxyPoolsPage);
		await screen.findByRole('table');

		expectGlyphAndLabel(screen.getByRole('button', { name: 'Add a proxy' }), 'Add a proxy');
	});

	it('carries the add glyph on the empty state "Add a proxy" control too', async () => {
		stubProxies({ pool: [] });
		render(ProxyPoolsPage);

		expectGlyphAndLabel(await screen.findByRole('button', { name: 'Add a proxy' }), 'Add a proxy');
	});

	it('carries the batch-add glyph on "Add several at once"', async () => {
		stubProxies({ pool: [proxyRow()] });
		render(ProxyPoolsPage);
		await screen.findByRole('table');

		expectGlyphAndLabel(
			screen.getByRole('button', { name: 'Add several at once' }),
			'Add several at once'
		);
	});

	it('carries the save glyph on "Save outbound settings"', async () => {
		stubProxies();
		render(ProxyPoolsPage);

		expectGlyphAndLabel(
			await screen.findByRole('button', { name: 'Save outbound settings' }),
			'Save outbound settings'
		);
	});

	it('renders exactly one refresh control: "Refresh now", with no plain "Refresh" beside it', async () => {
		stubProxies({ pool: [proxyRow()] });
		render(ProxyPoolsPage);
		await screen.findByRole('table');

		expect(screen.getByRole('button', { name: 'Refresh now' })).toBeTruthy();
		expect(screen.queryByRole('button', { name: 'Refresh' })).toBeNull();
	});

	it('renders edit, test, and delete as icon-only row actions named per row', async () => {
		stubProxies({ pool: [proxyRow()] });
		render(ProxyPoolsPage);
		const table = await screen.findByRole('table');
		const row = table.querySelector('tbody tr') as HTMLElement;

		for (const action of [
			'Edit Frankfurt egress',
			'Test Frankfurt egress',
			'Delete Frankfurt egress'
		]) {
			expectIconOnly(within(row).getByRole('button', { name: action }), action);
		}
	});

	it('gives the delete row action the danger colour, with no competing text colour', async () => {
		stubProxies({ pool: [proxyRow()] });
		render(ProxyPoolsPage);
		const table = await screen.findByRole('table');
		const row = table.querySelector('tbody tr') as HTMLElement;

		const button = within(row).getByRole('button', { name: 'Delete Frankfurt egress' });
		expect(button.className).toContain('text-[var(--color-danger)]');
		expect(button.className).not.toContain('text-[var(--color-text-muted)]');
	});
});
