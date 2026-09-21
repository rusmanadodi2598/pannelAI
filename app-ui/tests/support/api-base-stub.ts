// Shared fetch stub for the panel's API base route (docs/SPEC-UI/001-SPEC-UI.md §5.2).
//
// Two test files need the same answer: the dialog's own test drives the states around it, and the header
// test opens the dialog through its control, which reads the address as soon as it opens. The stub records
// every read, because the dialog reads on each open and that is a rule rather than an accident.
//
// The address it serves is deliberately on another host. A dialog that fell back to `location.origin` would
// pass against a stub that served the panel's own origin, and that fallback is the defect this route exists
// to remove.

import { vi } from 'vitest';

/** The address these tests serve, which no browser in the suite is running on. */
export const BASE_URL = 'http://gw.test:9090/api/v1';

export type ApiBaseStubOptions = {
	baseUrl?: string;
	status?: number;
	message?: string;
	/** A 200 whose body is not the shape the schema demands, for the drift path. */
	body?: unknown;
	/** Holds the answer until `release()` is called, which is how the loading state is observed. */
	hold?: boolean;
};

export type ApiBaseStub = {
	/** Every read the dialog made, in order. */
	reads: string[];
	/** The status the next read answers with, so a retry can be told from a repeat. */
	status: number;
	/** A body to answer with instead of the address, so the drift path is expressible and removable. */
	body: unknown;
	release: () => void;
};

export function stubApiBase(options: ApiBaseStubOptions = {}): ApiBaseStub {
	const reads: string[] = [];
	let release = (): void => {};

	const gate = options.hold
		? new Promise<void>((resolve) => {
				release = resolve;
			})
		: Promise.resolve();

	const stub: ApiBaseStub = {
		reads,
		status: options.status ?? 200,
		body: options.body,
		release
	};

	vi.stubGlobal('fetch', async (input: unknown) => {
		reads.push(String(input));
		await gate;

		const headers = { 'content-type': 'application/json' };

		if (stub.status !== 200) {
			return new Response(
				JSON.stringify({
					error: {
						code: 'INTERNAL_ERROR',
						message: options.message ?? 'The address is unavailable.'
					}
				}),
				{ status: stub.status, headers }
			);
		}

		const payload = stub.body ?? { base_url: options.baseUrl ?? BASE_URL };
		return new Response(JSON.stringify(payload), { status: 200, headers });
	});

	return stub;
}
