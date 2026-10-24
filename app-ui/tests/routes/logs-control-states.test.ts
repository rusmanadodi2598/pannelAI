// Every control the Logs screen renders carries a pressed colour, not only a hover one (DESIGN.md §3.5).
//
// The screen asked the same question eleven times with a class written at the call site: the pointer to
// `/console-log`, the filter commit, the clear, the purge, the row's open, the drawer's close, the two page
// steps, and the three actions the empty and failed states offer. None of them painted under a press, and
// the pointer link carried no state rule at all, so on a touch layout it answered nothing whatsoever.

import { cleanup, fireEvent, render, screen } from '@testing-library/svelte';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import LogsPage from '../../src/routes/logs/+page.svelte';
import { visit } from '../support/page.svelte';
import { stubLogs } from '../support/logs-stub';
import { SvelteURLSearchParams } from 'svelte/reactivity';
import { expectControlStates, expectEveryControlHasStates } from '../support/control-states';

vi.mock('$app/state', async () => {
	const { pageState: page } = await import('../support/page.svelte');
	return { page };
});

vi.mock('$app/paths', () => ({ resolve: (path: string) => path }));

vi.mock('$app/navigation', async () => {
	const { pageState: page, queryOf: query } = await import('../support/page.svelte');
	return {
		goto: (url: string) => {
			page.url = { pathname: '/logs', searchParams: new SvelteURLSearchParams(query(url)) };
			return Promise.resolve();
		}
	};
});

function controls(): HTMLElement[] {
	return Array.from(document.querySelectorAll('button, a[data-slot="button"]'));
}

describe('Logs control states', () => {
	beforeEach(() => {
		visit('/logs');
	});

	afterEach(() => {
		cleanup();
		vi.unstubAllGlobals();
	});

	it('gives every control on the requests tab a pressed colour', async () => {
		stubLogs();
		render(LogsPage);
		await screen.findByRole('table');

		const seen = controls();
		// The pointer to the console, the refresh, the filter commit, the purge, the row's open, the two
		// page steps, and the tab buttons the shell adds.
		expect(seen.length).toBeGreaterThanOrEqual(6);
		expectEveryControlHasStates(seen, 'the Logs screen');
	});

	it('keeps the pointer a link, and gives it the states anyway', async () => {
		stubLogs();
		render(LogsPage);
		await screen.findByRole('table');

		const link = screen.getByRole('link', { name: 'Console log' });
		expect(link.getAttribute('href')).toBe('/console-log');
		expectControlStates(link);
	});

	it('steps the purge along the danger ramp rather than washing it in ink', async () => {
		// A control that carries a colour carries its own ramp: an ink wash under a danger label measures
		// 4.17:1 on a raised panel, and the label is what says this one deletes.
		stubLogs();
		render(LogsPage);
		await screen.findByRole('table');

		const purge = screen.getByRole('button', { name: 'Purge captured logs' });
		expect(purge.className).toContain('text-[var(--color-danger)]');
		expect(purge.className).toContain('hover:text-[var(--color-danger-hover)]');
		expect(purge.className).toContain('active:text-[var(--color-danger-press)]');
		expect(purge.className).toContain('border-[var(--color-danger)]');
	});

	it('gives the drawer its close control on the scale', async () => {
		stubLogs();
		render(LogsPage);
		await screen.findByRole('table');

		await fireEvent.click(screen.getByRole('button', { name: 'Open' }));
		expectControlStates(await screen.findByRole('button', { name: 'Close' }));
	});

	it('gives the empty window its widen action', async () => {
		stubLogs({ records: [], total: 0 });
		render(LogsPage);

		expectControlStates(await screen.findByRole('button', { name: 'Look back 60 days' }));
	});

	it('gives a failed read its retry', async () => {
		stubLogs({ status: 500 });
		render(LogsPage);

		expectControlStates(await screen.findByRole('button', { name: 'Try again' }));
	});
});
