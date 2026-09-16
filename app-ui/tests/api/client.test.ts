// Tests for the HTTP client (src/lib/api/client.ts).
//
// The client is the only place that talks HTTP, so every outcome an operator can meet is a row in one
// table: a parsed body, a tolerated additive field, an empty 204, a mapped error envelope, a contract
// drift, a rejected body, an expired session, and a transport failure. Parsing itself is covered in
// tests/api/parse.test.ts.

import { afterEach, describe, expect, it, vi } from 'vitest';
import { z } from 'zod';
import { apiRequest, onUnauthorized, type RequestOptions } from '$lib/api/client';
import { ApiError } from '$lib/api/errors';
import { emptyResponse } from '$lib/schemas/primitives';

const schemaThing = z.object({ id: z.string(), count: z.number().int().min(0) });

type Payload = Record<string, unknown>;

function jsonResponse(status: number, body: unknown): Response {
	return new Response(JSON.stringify(body), {
		status,
		headers: { 'content-type': 'application/json' }
	});
}

type Outcome =
	| { kind: 'parsed'; count?: number }
	| { kind: 'error'; code: string; messageIncludes?: string }
	| { kind: 'validation' }
	| { kind: 'network' }
	| { kind: 'unauthorized' };

type ClientCase = {
	name: string;
	answer: () => Response | Promise<Response>;
	body?: Payload;
	bodySchema?: z.ZodType<Payload>;
	schema?: z.ZodType<Payload>;
	outcome: Outcome;
};

const cases: ClientCase[] = [
	{
		name: 'parses a successful body',
		answer: () => jsonResponse(200, { id: 'a', count: 2 }),
		outcome: { kind: 'parsed', count: 2 }
	},
	{
		name: 'tolerates an additive field',
		answer: () => jsonResponse(200, { id: 'a', count: 3, note: 'new' }),
		outcome: { kind: 'parsed', count: 3 }
	},
	{
		name: 'accepts an empty 204 response',
		answer: () => new Response(null, { status: 204 }),
		schema: emptyResponse,
		outcome: { kind: 'parsed' }
	},
	{
		name: 'maps a conflict envelope',
		answer: () => jsonResponse(409, { error: { code: 'CONFLICT', message: 'In use.' } }),
		outcome: { kind: 'error', code: 'CONFLICT', messageIncludes: 'In use.' }
	},
	{
		name: 'falls back when the API message is empty',
		answer: () => jsonResponse(404, { error: { code: 'NOT_FOUND', message: '  ' } }),
		outcome: { kind: 'error', code: 'NOT_FOUND', messageIncludes: 'no longer exists' }
	},
	{
		name: 'reports a missing field as drift',
		answer: () => jsonResponse(200, { id: 'a', total: 1 }),
		outcome: { kind: 'error', code: 'DRIFT', messageIncludes: 'count' }
	},
	{
		name: 'signals session expiry to the store',
		answer: () => jsonResponse(401, { error: { code: 'UNAUTHORIZED', message: 'expired' } }),
		outcome: { kind: 'unauthorized' }
	},
	{
		name: 'rejects an invalid body before sending',
		answer: () => jsonResponse(200, { id: 'a', count: 1 }),
		body: { name: '' },
		bodySchema: z.strictObject({ name: z.string().min(1, { message: 'A name is required.' }) }),
		outcome: { kind: 'validation' }
	},
	{
		name: 'reports a transport failure as a network error',
		answer: () => {
			throw new Error('connection refused');
		},
		outcome: { kind: 'network' }
	}
];

const CODE_BY_KIND = {
	validation: 'VALIDATION_ERROR',
	network: 'NETWORK',
	unauthorized: 'UNAUTHORIZED'
};

describe('apiRequest', () => {
	afterEach(() => {
		vi.unstubAllGlobals();
		onUnauthorized(undefined);
	});

	for (const testCase of cases) {
		it(testCase.name, async () => {
			const fetchMock = vi.fn(async () => testCase.answer());
			vi.stubGlobal('fetch', fetchMock);

			const handler = vi.fn();
			onUnauthorized(handler);

			const options: RequestOptions<Payload, Payload> = {
				method: 'GET',
				path: '/things',
				schema: testCase.schema ?? schemaThing,
				body: testCase.body,
				bodySchema: testCase.bodySchema
			};

			const result = await apiRequest(options);
			const expected = testCase.outcome;

			expect(fetchMock).toHaveBeenCalledTimes(expected.kind === 'validation' ? 0 : 1);
			expect(handler).toHaveBeenCalledTimes(expected.kind === 'unauthorized' ? 1 : 0);

			if (expected.kind === 'parsed') {
				expect(result.ok).toBe(true);
				if (result.ok) expect(result.data.count).toBe(expected.count);
				return;
			}

			expect(result.ok).toBe(false);
			if (result.ok) return;

			expect(result.error).toBeInstanceOf(ApiError);

			const expectedCode = expected.kind === 'error' ? expected.code : CODE_BY_KIND[expected.kind];

			expect(result.error.code).toBe(expectedCode);

			if (expected.kind === 'error' && expected.messageIncludes) {
				expect(result.error.message).toContain(expected.messageIncludes);
			}
		});
	}
});
