// Every control on Proxy Pools carries the states (DESIGN.md §3.5).
//
// This is the screen where the audit found more than missing states: the bulk bar's commit button painted
// itself with `--color-primary-contrast` and its tag field with `--color-surface-1`, and neither token is
// declared anywhere, so the button inherited the page's text colour, which is near-white on the coral fill
// in dark. Both readings are pinned here so they cannot come back as a typo again.

import { cleanup, fireEvent, render, screen } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import BulkActionBar from '../../src/lib/components/BulkActionBar.svelte';
import ProxyPoolsPage from '../../src/routes/proxy-pools/+page.svelte';
import { expectEveryControlHasStates } from '../support/control-states';
import { proxyRow, stubProxies } from '../support/proxy-stub';

function controls(): HTMLElement[] {
	return Array.from(document.querySelectorAll('button'));
}

describe('Proxy Pools control states', () => {
	afterEach(() => {
		cleanup();
		vi.unstubAllGlobals();
	});

	it('gives the toolbar, the table actions, and the outbound card the same states', async () => {
		stubProxies({ pool: [proxyRow()] });
		render(ProxyPoolsPage);
		await screen.findByRole('button', { name: /Add a proxy/ });

		expectEveryControlHasStates(controls(), 'Proxy Pools');
	});

	it('gives the bulk bar its states once a row is selected', async () => {
		stubProxies({ pool: [proxyRow()] });
		render(ProxyPoolsPage);
		const rowSelect = await screen.findByRole('checkbox', { name: /Select/ });
		await fireEvent.click(rowSelect);

		const selected = controls();
		expect(selected.length, 'the bulk bar rendered no controls').toBeGreaterThan(4);
		expectEveryControlHasStates(selected, 'Proxy Pools bulk bar');
	});

	it('paints the bulk commit with the accent pair, not with a token that does not exist', async () => {
		// The tag dialog lives on the bar itself, which is where both dead token references sat. The proxy
		// surface has no tags, so the bar is rendered directly rather than through this screen.
		render(BulkActionBar, {
			props: {
				kind: 'keys',
				unit: 'key',
				selected: 2,
				total: 9,
				scope: 'this provider',
				ondisable: () => {},
				onenable: () => {},
				ondelete: () => {},
				onclear: () => {},
				ontag: () => {},
				ontoggleall: () => {}
			}
		});

		// The bar names every control with the count it acts on, so the tag action is "Tag 2 keys".
		await fireEvent.click(screen.getByRole('button', { name: 'Tag 2 keys' }));
		const apply = await screen.findByRole('button', { name: 'Apply the tag' });

		expect(apply.className).toContain('bg-[var(--color-accent)]');
		expect(apply.className).toContain('text-[var(--color-accent-text)]');
		expect(apply.className).not.toMatch(/primary-contrast|color-surface-1/);
		expect(
			screen.getByLabelText(/Label to put on these/).className,
			'the tag field lost its ground'
		).toContain('bg-[var(--color-surface)]');
	});
});
