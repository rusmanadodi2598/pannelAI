// The console screen's fetch harness, shared by the tests that render it.
//
// The buffer is server-side, so a clear has to change what the *next read* returns rather than what the
// page holds. The stub models that: lines come from `lines` until the DELETE lands, and from
// `afterClear` afterwards.

import { vi } from 'vitest';

export type ConsoleStub = {
	/** Every call as `METHOD /logs/console`, in the order the screen asked for it. */
	requested: string[];
	lines: string[];
	maxRecords: number;
	/** What the ring holds after the clear, which is the point of testing a server-side delete. */
	afterClear: string[];
	/** Status the GET answers with, for the failed-read states. */
	status: number;
	/** Status the DELETE answers with, for the failed-write state inside the dialog. */
	clearStatus: number;
};

export function stubConsole(overrides: Partial<ConsoleStub> = {}): ConsoleStub {
	const stub: ConsoleStub = {
		requested: [],
		lines: ['gateway booted', 'request req_1 served'],
		maxRecords: 1000,
		afterClear: [],
		status: 200,
		clearStatus: 204,
		...overrides
	};

	let cleared = false;

	vi.stubGlobal('fetch', async (input: unknown, init?: RequestInit) => {
		const method = init?.method ?? 'GET';
		stub.requested.push(`${method} /logs/console`);

		if (method === 'DELETE') {
			if (stub.clearStatus !== 204) {
				return json(
					{ error: { code: 'INTERNAL_ERROR', message: 'buffer is locked' } },
					stub.clearStatus
				);
			}
			cleared = true;
			return new Response(null, { status: 204 });
		}

		if (stub.status !== 200) {
			return json(
				{ error: { code: 'INTERNAL_ERROR', message: 'The log store is unreachable.' } },
				stub.status
			);
		}

		return json(
			{ lines: cleared ? stub.afterClear : stub.lines, max_records: stub.maxRecords },
			stub.status
		);
	});

	return stub;
}

function json(body: unknown, status: number): Response {
	return new Response(JSON.stringify(body), {
		status,
		headers: { 'content-type': 'application/json' }
	});
}

export function callsOf(stub: ConsoleStub, method: string): string[] {
	return stub.requested.filter((call) => call.startsWith(method));
}
