// Combo delete tests (docs/SPEC-UI/001-SPEC-UI.md §6.4).
//
// `DELETE /combos/{id}` answers CONFLICT when an alias still targets the combo's name, and the message names
// that alias. The panel renders that answer as the reason rather than as a generic failure, and it names no
// screen: the alias set has no screen of its own, so a sentence pointing at one would be a dead end.
//
// The other half of the test is the one that keeps the copy honest: only a CONFLICT means the combo is still
// referenced, so a server failure must not read as though it did.

import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/svelte';
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

/** One provider row as the picker's provider read answers it. */
function providerRow(overrides: Record<string, unknown> = {}): Record<string, unknown> {
	return {
		id: 'openai',
		name: 'OpenAI',
		category: 'apikey',
		auth_type: 'bearer',
		auth_modes: ['api_key'],
		has_oauth: false,
		no_auth: false,
		routability: 'native',
		endpoint_count: 1,
		status_summary: { total: 1, active: 1, disabled: 0, error: 0, rate_limited: 0 },
		...overrides
	};
}

/** Serves the tab's two reads and answers the delete with whatever a test asks for. */
function stubFetch(deleteStatus: number, deleteCode: string, deleteMessage: string): string[] {
	const requested: string[] = [];

	vi.stubGlobal('fetch', async (input: unknown, init?: RequestInit) => {
		const url = String(input);
		requested.push(url);
		const json = (payload: unknown, status = 200): Response =>
			new Response(JSON.stringify(payload), {
				status,
				headers: { 'content-type': 'application/json' }
			});

		if (init?.method === 'DELETE') {
			return json({ error: { code: deleteCode, message: deleteMessage } }, deleteStatus);
		}
		if (url.includes('/models/catalog')) {
			return json({
				data: [
					{
						id: 'openai/gpt-4o',
						provider_id: 'openai',
						model_id: 'gpt-4o',
						display_name: 'GPT-4o',
						kind: 'chat',
						capabilities: ['vision'],
						source: 'registry'
					},
					{
						id: 'oczen/mimo-v2.6-flash-free',
						provider_id: 'oczen',
						model_id: 'mimo-v2.6-flash-free',
						display_name: 'MiMo v2.6',
						kind: 'chat',
						capabilities: [],
						source: 'custom'
					}
				]
			});
		}
		// The picker's other read: only `openai` carries an endpoint, so `oczen` must not be offered.
		if (url.includes('/providers')) {
			return json({
				data: [
					providerRow({ id: 'openai', name: 'OpenAI', endpoint_count: 1 }),
					providerRow({ id: 'oczen', name: 'OpenCode Zen Free', endpoint_count: 0 })
				],
				meta: { page: 1, per_page: 100, total: 2 }
			});
		}
		if (url.includes('/combos')) {
			return json({ data: [combo], meta: { page: 1, per_page: 25, total: 1 } });
		}
		return json({ error: { code: 'NOT_FOUND', message: 'No route matches that request.' } }, 404);
	});

	return requested;
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
	it('names the alias that still references it, and points at no screen', async () => {
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
		// The alias set has no screen of its own, so the refusal must not send the operator to one.
		expect(squashed(alert)).not.toContain('Aliases');
		expect(squashed(alert)).not.toContain('detail screen');
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

	it('re-reads the combo list when the operator asks for it', async () => {
		const requested = stubFetch(409, 'CONFLICT', 'alias fast still references combo daily');
		render(CombosTab);
		await waitFor(() => expect(screen.getByText('daily')).toBeTruthy());

		const combosReads = (): number =>
			requested.filter((url) => url.split('?')[0].endsWith('/combos')).length;
		const before = combosReads();

		await fireEvent.click(screen.getByRole('button', { name: 'Refresh now' }));

		// §8.6.2: the tab repeats the read rather than leaving a page reload as the only way to see a combo
		// another operator just added.
		await waitFor(() => expect(combosReads()).toBeGreaterThan(before));
	});
});

describe('the editor reference picker', () => {
	it('offers the refs of a configured provider and the combo names, and nothing from an unconfigured one', async () => {
		stubFetch(409, 'CONFLICT', 'alias fast still references combo daily');
		render(CombosTab);

		// The editor owns the picker, so it has to be open for the dialog to exist.
		await waitFor(() => expect(screen.getByRole('button', { name: 'New combo' })).toBeTruthy());
		await fireEvent.click(screen.getByRole('button', { name: 'New combo' }));
		await fireEvent.click(screen.getByRole('button', { name: 'Add models' }));

		const dialog = await screen.findByRole('dialog');
		expect(within(dialog).getByRole('button', { name: 'GPT-4o' })).toBeTruthy();
		expect(within(dialog).getByRole('button', { name: 'daily' })).toBeTruthy();
		// `oczen` has no endpoint row, so its 82 catalog rows are not offered: the reference's rule is
		// providers that are active now, and this gateway cannot route a provider without an endpoint.
		expect(within(dialog).queryByRole('button', { name: 'MiMo v2.6' })).toBeNull();
	});
});
