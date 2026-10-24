// Shared fixtures for the Token Saver screen tests (docs/SPEC-UI/001-SPEC-UI.md §6.7).
//
// The read and the write are stubbed separately on purpose: a test that makes both fail cannot tell "the
// load broke" from "the save broke". The stub records every PUT body, because the rules worth checking
// here are about what the panel sent, not only about what it rendered.
//
// Reading a control's state is `tests/support/dom.ts`, which the component tests share.

import { vi } from 'vitest';
import type { TokenSaver } from '$lib/schemas/token-saver';

export function tokenSaverDocument(overrides: Partial<TokenSaver> = {}): TokenSaver {
	return {
		rtk: { enabled: false, filters: [] },
		headroom: { enabled: false, url: '', compress_user_messages: false },
		ponytail: { enabled: false, level: 'full' },
		...overrides
	};
}

export type TokenSaverStub = {
	puts: Record<string, unknown>[];
	config: TokenSaver;
	readStatus: number;
	writeStatus: number;
	/** Holds the PUT open, for the one-write-at-a-time rule. */
	putDelayMs: number;
};

export function stubTokenSaver(overrides: Partial<TokenSaverStub> = {}): TokenSaverStub {
	const stub: TokenSaverStub = {
		puts: [],
		config: tokenSaverDocument(),
		readStatus: 200,
		writeStatus: 200,
		putDelayMs: 0,
		...overrides
	};

	const envelope = (code: string, message: string): string =>
		JSON.stringify({ error: { code, message } });

	vi.stubGlobal('fetch', async (_input: unknown, init?: RequestInit) => {
		const method = init?.method ?? 'GET';

		if (method === 'PUT') {
			const body = JSON.parse(String(init?.body ?? '{}')) as TokenSaver;
			stub.puts.push(body);

			if (stub.putDelayMs > 0) {
				await new Promise((resolve) => setTimeout(resolve, stub.putDelayMs));
			}

			if (stub.writeStatus !== 200) {
				return new Response(envelope('VALIDATION_ERROR', 'The gateway refused this value.'), {
					status: stub.writeStatus,
					headers: { 'content-type': 'application/json' }
				});
			}

			// A successful replace is what the next read answers with, which is what the page re-reads.
			stub.config = body;
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
