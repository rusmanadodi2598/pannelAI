// Tests for response parsing and the HTTP client.
//
// The client is the only place that talks HTTP, so these tests cover the three outcomes an operator
// can meet: a parsed body, a contract drift, and a mapped error.

import { afterEach, describe, expect, it, vi } from 'vitest';
import { z } from 'zod';
import { parseResponse, unknownTopLevelKeys } from '$lib/api/parse';
import { apiRequest, onUnauthorized } from '$lib/api/client';
import { ApiError } from '$lib/api/errors';
import { emptyResponse, type EmptyResponse } from '$lib/schemas/primitives';

const schemaThing = z.object({ id: z.string(), count: z.number().int().min(0) });

describe('unknownTopLevelKeys', () => {
	const cases = [
		{ name: 'no drift', input: { id: 'a', count: 1 }, expected: [] },
		{ name: 'one added field', input: { id: 'a', count: 1, extra: true }, expected: ['extra'] },
		{ name: 'an array payload', input: [1, 2], expected: [] },
		{ name: 'a null payload', input: null, expected: [] },
		{ name: 'a primitive payload', input: 'text', expected: [] }
	];

	for (const testCase of cases) {
		it(`reports ${testCase.name}`, () => {
			expect(unknownTopLevelKeys(schemaThing, testCase.input)).toEqual(testCase.expected);
		});
	}
});

describe('parseResponse', () => {
	const cases = [
		{ name: 'a valid body', input: { id: 'a', count: 0 }, ok: true },
		{ name: 'an additive field', input: { id: 'a', count: 3, note: 'new' }, ok: true },
		{ name: 'a renamed field', input: { id: 'a', total: 3 }, ok: false },
		{ name: 'a wrong type', input: { id: 'a', count: 'three' }, ok: false },
		{ name: 'a negative count', input: { id: 'a', count: -3 }, ok: false },
		{ name: 'a missing body', input: null, ok: false }
	];

	for (const testCase of cases) {
		it(`${testCase.ok ? 'accepts' : 'rejects'} ${testCase.name}`, () => {
			expect(parseResponse(schemaThing, testCase.input).ok).toBe(testCase.ok);
		});
	}

	it('names the offending path on failure', () => {
		const result = parseResponse(schemaThing, { id: 'a', total: 3 });
		expect(result.ok).toBe(false);
		if (!result.ok) expect(result.path).toBe('count');
	});

	it('reports drift alongside parsed data', () => {
		const result = parseResponse(schemaThing, { id: 'a', count: 1, extra: 1 });
		expect(result.ok).toBe(true);
		if (result.ok) expect(result.drift).toEqual(['extra']);
	});
});

function jsonResponse(
	status: number,
	body: unknown,
	headers: Record<string, string> = {}
): Response {
	return new Response(JSON.stringify(body), {
		status,
		headers: { 'content-type': 'application/json', ...headers }
	});
}

describe('apiRequest', () => {
	afterEach(() => {
		vi.unstubAllGlobals();
		onUnauthorized(undefined);
	});

	it('parses a successful body', async () => {
		vi.stubGlobal(
			'fetch',
			vi.fn(async () => jsonResponse(200, { id: 'a', count: 2 }))
		);

		const result = await apiRequest<void, { id: string; count: number }>({
			method: 'GET',
			path: '/things',
			schema: schemaThing
		});

		expect(result.ok).toBe(true);
		if (result.ok) expect(result.data.count).toBe(2);
	});

	it('maps a management error envelope', async () => {
		vi.stubGlobal(
			'fetch',
			vi.fn(async () => jsonResponse(409, { error: { code: 'CONFLICT', message: 'In use.' } }))
		);

		const result = await apiRequest<void, { id: string }>({
			method: 'DELETE',
			path: '/things/a',
			schema: schemaThing
		});

		expect(result.ok).toBe(false);
		if (!result.ok) {
			expect(result.error).toBeInstanceOf(ApiError);
			expect(result.error.code).toBe('CONFLICT');
			expect(result.error.message).toBe('In use.');
		}
	});

	it('falls back to a panel message when the API message is empty', async () => {
		vi.stubGlobal(
			'fetch',
			vi.fn(async () => jsonResponse(404, { error: { code: 'NOT_FOUND', message: '  ' } }))
		);

		const result = await apiRequest<void, { id: string }>({
			method: 'GET',
			path: '/things/missing',
			schema: schemaThing
		});

		expect(result.ok).toBe(false);
		if (!result.ok) expect(result.error.message).toContain('no longer exists');
	});

	it('reports contract drift without throwing', async () => {
		vi.stubGlobal(
			'fetch',
			vi.fn(async () => jsonResponse(200, { id: 'a', total: 1 }))
		);

		const result = await apiRequest<void, { id: string }>({
			method: 'GET',
			path: '/things',
			schema: schemaThing
		});

		expect(result.ok).toBe(false);
		if (!result.ok) {
			expect(result.error.code).toBe('DRIFT');
			expect(result.error.message).toContain('count');
		}
	});

	it('signals session expiry to the store', async () => {
		const handler = vi.fn();
		onUnauthorized(handler);
		vi.stubGlobal(
			'fetch',
			vi.fn(async () => jsonResponse(401, { error: { code: 'UNAUTHORIZED', message: 'expired' } }))
		);

		await apiRequest<void, { id: string }>({ method: 'GET', path: '/things', schema: schemaThing });

		expect(handler).toHaveBeenCalledOnce();
	});

	it('rejects an invalid body before sending', async () => {
		const fetchMock = vi.fn();
		vi.stubGlobal('fetch', fetchMock);

		const result = await apiRequest<{ name: string }, { id: string }>({
			method: 'POST',
			path: '/things',
			schema: schemaThing,
			body: { name: '' },
			bodySchema: z.strictObject({ name: z.string().min(1, { message: 'A name is required.' }) })
		});

		expect(result.ok).toBe(false);
		expect(fetchMock).not.toHaveBeenCalled();
	});

	it('accepts an empty 204 response', async () => {
		vi.stubGlobal(
			'fetch',
			vi.fn(async () => new Response(null, { status: 204 }))
		);

		const result = await apiRequest<void, EmptyResponse>({
			method: 'DELETE',
			path: '/things/a',
			schema: emptyResponse
		});

		expect(result.ok).toBe(true);
	});

	it('reports a transport failure as a network error', async () => {
		vi.stubGlobal(
			'fetch',
			vi.fn(async () => {
				throw new Error('connection refused');
			})
		);

		const result = await apiRequest<void, { id: string }>({
			method: 'GET',
			path: '/things',
			schema: schemaThing
		});

		expect(result.ok).toBe(false);
		if (!result.ok) expect(result.error.code).toBe('NETWORK');
	});
});
