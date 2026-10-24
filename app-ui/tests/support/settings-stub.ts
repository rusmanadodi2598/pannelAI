// The settings screen's fetch harness, shared by the tests that render it.
//
// The stub answers a read and a write differently on purpose: a test that makes both fail cannot tell
// "the load broke" from "the save broke", and the rule under test is usually which group a write named.

import { fireEvent, screen } from '@testing-library/svelte';
import { vi } from 'vitest';
import { settingsDocument } from './settings-document';

type Stub = {
	patches: Record<string, unknown>[];
	settings: Record<string, unknown>;
	readStatus: number;
	writeStatus: number;
};

export function stubSettings(overrides: Partial<Stub> = {}): Stub {
	const stub: Stub = {
		patches: [],
		settings: settingsDocument(),
		readStatus: 200,
		writeStatus: 200,
		...overrides
	};

	const envelope = (code: string, message: string): string =>
		JSON.stringify({ error: { code, message } });

	vi.stubGlobal('fetch', async (input: unknown, init?: RequestInit) => {
		const method = init?.method ?? 'GET';

		if (method === 'PATCH') {
			stub.patches.push(JSON.parse(String(init?.body ?? '{}')));

			if (stub.writeStatus !== 200) {
				return new Response(envelope('VALIDATION_ERROR', 'The gateway refused this value.'), {
					status: stub.writeStatus,
					headers: { 'content-type': 'application/json' }
				});
			}

			return new Response(JSON.stringify(stub.settings), {
				status: 200,
				headers: { 'content-type': 'application/json' }
			});
		}

		if (stub.readStatus !== 200) {
			return new Response(envelope('INTERNAL_ERROR', 'The settings store is unreachable.'), {
				status: stub.readStatus,
				headers: { 'content-type': 'application/json' }
			});
		}

		return new Response(JSON.stringify(stub.settings), {
			status: 200,
			headers: { 'content-type': 'application/json' }
		});
	});

	return stub;
}

export async function openTab(name: string): Promise<void> {
	await screen.findByRole('tablist');
	await fireEvent.click(screen.getByRole('tab', { name }));
	await screen.findByRole('tabpanel');
}

/**
 * One row of the failure-class editor, addressed by the class it names.
 *
 * Every row carries the same three control labels, `Action`, `Ceiling (seconds)` and `Flat cooldown
 * (seconds)`, so the row is found as a group and its controls are read from inside it. A query aimed at the
 * whole screen would match six elements and throw on the first.
 */
export async function classRow(title: string): Promise<HTMLElement> {
	return await screen.findByRole('group', { name: title });
}
