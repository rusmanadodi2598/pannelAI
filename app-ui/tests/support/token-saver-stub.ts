// Shared fixtures for the Token Saver screen tests (docs/SPEC-UI/001-SPEC-UI.md §6.7).
//
// The read and the write are stubbed separately on purpose: a test that makes both fail cannot tell "the
// load broke" from "the save broke". The stub records every PUT body, because the rules worth checking
// here are about what the panel sent, not only about what it rendered.
//
// The panel's tests run on plain Vitest assertions, without a jest-dom matcher layer, so a control's
// state is read through the element rather than through a custom matcher.

import { vi } from 'vitest';

export function tokenSaverDocument(
	overrides: Record<string, unknown> = {}
): Record<string, unknown> {
	return {
		rtk: { enabled: false, filters: [] },
		headroom: { enabled: false, url: '', compress_user_messages: false },
		ponytail: { enabled: false, level: 'full' },
		...overrides
	};
}

export type TokenSaverStub = {
	puts: Record<string, unknown>[];
	config: Record<string, unknown>;
	readStatus: number;
	writeStatus: number;
};

export function stubTokenSaver(overrides: Partial<TokenSaverStub> = {}): TokenSaverStub {
	const stub: TokenSaverStub = {
		puts: [],
		config: tokenSaverDocument(),
		readStatus: 200,
		writeStatus: 200,
		...overrides
	};

	const envelope = (code: string, message: string): string =>
		JSON.stringify({ error: { code, message } });

	vi.stubGlobal('fetch', async (_input: unknown, init?: RequestInit) => {
		const method = init?.method ?? 'GET';

		if (method === 'PUT') {
			stub.puts.push(JSON.parse(String(init?.body ?? '{}')));

			if (stub.writeStatus !== 200) {
				return new Response(envelope('VALIDATION_ERROR', 'The gateway refused this value.'), {
					status: stub.writeStatus,
					headers: { 'content-type': 'application/json' }
				});
			}

			return new Response(JSON.stringify(stub.config), {
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

		return new Response(JSON.stringify(stub.config), {
			status: 200,
			headers: { 'content-type': 'application/json' }
		});
	});

	return stub;
}

export function checked(element: HTMLElement): boolean {
	return (element as HTMLInputElement).checked;
}

export function value(element: HTMLElement): string {
	return (element as HTMLInputElement | HTMLSelectElement).value;
}

export function text(element: HTMLElement): string {
	return element.textContent ?? '';
}
