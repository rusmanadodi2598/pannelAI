// The panel's only HTTP entry point.
//
// Components never call fetch (docs/SPEC-UI/001-SPEC-UI.md §10.1), so validation, error mapping, and
// drift reporting exist once. Session expiry is not handled here: the client reports UNAUTHORIZED and
// registers a handler, and the session store decides to route to the login screen, which keeps
// routing out of the transport layer.

import type { z } from 'zod';
import { ApiError, errorFromPayload } from './errors';
import { parseResponse, type ParseResult } from './parse';

const API_PREFIX = '/api/v1';

/**
 * A call's outcome.
 *
 * `refusal` carries a failed route's own extra body, for the one family of routes that has one: a refused
 * batch reports every row by index (SPEC-API §8.1), which is what lets a message land on the row it names.
 * It is `never` unless the caller declared a `refusalSchema`, so a screen cannot read a field its route does
 * not send.
 */
export type ApiResult<T, R = never> =
	{ ok: true; data: T } | { ok: false; error: ApiError; refusal?: R };

export type RequestOptions<B, T, R = never> = {
	method: 'GET' | 'POST' | 'PATCH' | 'PUT' | 'DELETE';
	path: string;
	schema: z.ZodType<T>;
	body?: B;
	bodySchema?: z.ZodType<B>;
	query?: Record<string, string | number | boolean | undefined>;
	/** A refusal body this route carries beyond the §8 envelope, parsed only when the answer is not ok. */
	refusalSchema?: z.ZodType<R>;
};

let unauthorizedHandler: (() => void) | undefined;

export function onUnauthorized(handler: (() => void) | undefined): void {
	unauthorizedHandler = handler;
}

/**
 * Reports a session expiry the way this client does, for a caller that does its own fetch.
 *
 * One module does: the playground calls the panel's own routes and reads a failure envelope of its own,
 * so it cannot go through `apiRequest`, but an expired session must still sign the operator out once,
 * in one place (SPEC-UI §8.1).
 */
export function reportUnauthorized(status: number): void {
	if (status === 401) unauthorizedHandler?.();
}

export async function apiRequest<B, T, R = never>(
	options: RequestOptions<B, T, R>
): Promise<ApiResult<T, R>> {
	// The body starts as whatever the caller passed, because `bodySchema` validates a body rather than
	// deciding whether one is sent. Assigning it only inside the branch below would drop the body of every
	// route that has no schema, and the far end would see an empty request rather than the panel reporting a
	// mistake it could have caught.
	let body: unknown = options.body;

	if (options.bodySchema && options.body !== undefined) {
		const parsed = options.bodySchema.safeParse(options.body);
		if (!parsed.success) {
			const issue = parsed.error.issues[0];
			return {
				ok: false,
				error: new ApiError(0, 'VALIDATION_ERROR', `${issue.path.join('.')}: ${issue.message}`)
			};
		}
		body = parsed.data;
	}

	let response: Response;
	try {
		response = await fetch(buildUrl(options.path, options.query), {
			method: options.method,
			headers: buildHeaders(body !== undefined),
			body: body === undefined ? undefined : JSON.stringify(body),
			credentials: 'same-origin'
		});
	} catch (cause) {
		const detail = cause instanceof Error ? cause.message : 'the request failed';
		return { ok: false, error: new ApiError(0, 'NETWORK', detail) };
	}

	if (response.status === 401 || response.status === 403) {
		reportUnauthorized(response.status);
	}

	if (!response.ok) {
		// The body is read once and used twice: the envelope names the failure, and a batch route's refusal
		// carries each row's own verdict beside it (§8.1). A refusal that does not parse is dropped rather
		// than reported, because the envelope already states the failure the operator has to act on.
		const payload = await readJson(response);
		const refusal = options.refusalSchema?.safeParse(payload);

		return {
			ok: false,
			error: errorFromPayload(response, payload),
			refusal: refusal?.success ? refusal.data : undefined
		};
	}

	if (response.status === 204) {
		const empty = options.schema.safeParse({});
		if (empty.success) return { ok: true, data: empty.data };
		return { ok: false, error: new ApiError(204, 'DRIFT', 'Expected a body that was not sent.') };
	}

	const payload = await readJson(response);
	const result: ParseResult<T> = parseResponse(options.schema, payload);

	if (!result.ok) {
		return {
			ok: false,
			error: new ApiError(
				response.status,
				'DRIFT',
				`Unexpected response from the gateway at ${result.path}: ${result.message}`
			)
		};
	}

	if (result.drift.length > 0 && import.meta.env.DEV) {
		console.warn(`[drift] ${options.path} returned unknown keys: ${result.drift.join(', ')}`);
	}

	return { ok: true, data: result.data };
}

async function readJson(response: Response): Promise<unknown> {
	try {
		return await response.json();
	} catch {
		return null;
	}
}

function buildUrl(path: string, query?: RequestOptions<unknown, unknown>['query']): string {
	const url = new URL(`${API_PREFIX}${path}`, location.origin);
	for (const [key, value] of Object.entries(query ?? {})) {
		if (value !== undefined && value !== '') url.searchParams.set(key, String(value));
	}
	return url.pathname + url.search;
}

function buildHeaders(hasBody: boolean): Headers {
	const headers = new Headers({ accept: 'application/json' });
	if (hasBody) headers.set('content-type', 'application/json');
	return headers;
}
