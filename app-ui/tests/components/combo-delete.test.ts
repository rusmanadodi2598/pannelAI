// Combo delete tests (docs/SPEC-UI/001-SPEC-UI.md §6.4).
//
// `DELETE /combos/{id}` answers CONFLICT when an alias still targets the combo's name, and the message names
// that alias. §6.4 asks the screen to point at the fixed set: the set now has a table on the provider detail
// screen, but that route takes a provider id and this refusal names none, so the sentence names the place.
//
// The other half of the test is the one that keeps the copy honest: only a CONFLICT means the combo is still
// referenced, so a server failure must not read as though it did.

import { cleanup, render, screen, waitFor, within } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import CombosTab from '../../src/lib/components/CombosTab.svelte';
import { squashed } from '../support/dom';

const combo = {
	id: 'cmb_1',
	name: 'daily',
	strategy: 'fallback',
	sticky_limit: 1,
	judge_model: '',
	models: [{ ref: 'openai/gpt-4o', priority: 0 }],
	created_at: '2026-09-18T00:00:00Z',
	updated_at: '2026-09-18T00:00:00Z'
};

/** Serves the tab's three reads and answers the delete with whatever a test asks for. */
function stubFetch(deleteStatus: number, deleteCode: string, deleteMessage: string): void {
	vi.stubGlobal('fetch', async (input: unknown, init?: RequestInit) => {
		const url = String(input);
		const json = (payload: unknown, status = 200): Response =>
			new Response(JSON.stringify(payload), {
				status,
				headers: { 'content-type': 'application/json' }
			});

		if (init?.method === 'DELETE') {
			return json({ error: { code: deleteCode, message: deleteMessage } }, deleteStatus);
		}
		if (url.includes('/models/aliases')) {
			return json({ data: [{ alias: 'fast', target: 'openai/gpt-4o' }] });
		}
		if (url.includes('/models/catalog')) return json({ data: [] });
		if (url.includes('/combos')) {
			return json({ data: [combo], meta: { page: 1, per_page: 25, total: 1 } });
		}
		return json({ error: { code: 'NOT_FOUND', message: 'No route matches that request.' } }, 404);
	});
}

afterEach(() => {
	cleanup();
	vi.unstubAllGlobals();
});

async function openDeleteDialog(): Promise<HTMLElement> {
	await waitFor(() => expect(screen.getByRole('button', { name: 'Delete' })).toBeTruthy());
	screen.getByRole('button', { name: 'Delete' }).click();
	return screen.findByRole('dialog');
}

describe('deleting a combo', () => {
	it('names the alias that still references it, and where the alias set lives', async () => {
		stubFetch(409, 'CONFLICT', 'alias fast still references combo daily');
		render(CombosTab);

		const dialog = await openDeleteDialog();
		within(dialog).getByRole('button', { name: 'Delete the combo' }).click();

		await waitFor(() =>
			expect(screen.getByText(/alias fast still references combo daily/)).toBeTruthy()
		);
		const alert = within(screen.getByRole('dialog')).getByRole('alert');
		// Squashed, because the source line break inside the sentence is a newline in the DOM.
		expect(squashed(alert)).toContain('This combo is still referenced');
		expect(squashed(alert)).toContain(
			"The alias set is on any provider's detail screen, under Aliases."
		);
	});

	it('does not claim the combo is referenced when the failure was something else', async () => {
		stubFetch(500, 'INTERNAL_ERROR', 'The combo could not be removed.');
		render(CombosTab);

		const dialog = await openDeleteDialog();
		within(dialog).getByRole('button', { name: 'Delete the combo' }).click();

		await waitFor(() => expect(screen.getByText(/The combo could not be removed/)).toBeTruthy());
		const alert = within(screen.getByRole('dialog')).getByRole('alert');
		expect(squashed(alert)).not.toContain('still referenced');
		expect(squashed(alert)).not.toContain('Aliases');
	});

	it('cancels without a request', async () => {
		stubFetch(409, 'CONFLICT', 'alias fast still references combo daily');
		render(CombosTab);

		const dialog = await openDeleteDialog();
		within(dialog).getByRole('button', { name: 'Keep it' }).click();

		await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull());
		expect(screen.getByText('daily')).toBeTruthy();
	});
});

describe('the editor reference suggestions', () => {
	it('offers the alias names alongside the catalog ids, because a ref may be either', async () => {
		stubFetch(409, 'CONFLICT', 'alias fast still references combo daily');
		render(CombosTab);

		// The editor owns the list, so it has to be open for the datalist to exist.
		await waitFor(() => expect(screen.getByRole('button', { name: 'New combo' })).toBeTruthy());
		screen.getByRole('button', { name: 'New combo' }).click();

		await waitFor(() =>
			expect(document.querySelectorAll('#combo-ref-suggestions option').length).toBeGreaterThan(0)
		);
		const options = [...document.querySelectorAll('#combo-ref-suggestions option')].map((option) =>
			option.getAttribute('value')
		);
		expect(options).toContain('fast');
		expect(options).toContain('daily');
	});
});
