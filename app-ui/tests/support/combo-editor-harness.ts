// The editor fixtures the two combo-editor test files share (docs/SPEC-UI/001-SPEC-UI.md §6.4).
//
// One fixture builds the parsed combo and one stubs the save, because both files ask what the editor showed
// and what it sent, and a second copy of either would let the two answers drift.

import { fireEvent, screen } from '@testing-library/svelte';
import { vi } from 'vitest';
import { schemaCombo, type Combo } from '$lib/schemas/combo';

export function combo(overrides: Record<string, unknown> = {}): Combo {
	return schemaCombo.parse({
		id: 'cmb_1',
		name: 'daily',
		strategy: 'fallback',
		sticky_limit: 1,
		judge_model: '',
		models: [{ ref: 'openai/gpt-4o', priority: 0 }],
		created_at: '2026-09-18T00:00:00Z',
		updated_at: '2026-09-18T00:00:00Z',
		...overrides
	});
}

/** Records every request the editor makes, so a test can assert what was sent as well as what was shown. */
export function stubFetch(status = 201): { calls: { url: string; body: unknown }[] } {
	const calls: { url: string; body: unknown }[] = [];

	vi.stubGlobal('fetch', async (input: unknown, init?: RequestInit) => {
		calls.push({
			url: String(input),
			body: init?.body === undefined ? undefined : JSON.parse(String(init.body))
		});
		return new Response(JSON.stringify(combo()), {
			status,
			headers: { 'content-type': 'application/json' }
		});
	});

	return { calls };
}

export async function selectStrategy(value: string): Promise<void> {
	await fireEvent.change(screen.getByLabelText('Strategy'), { target: { value } });
}
