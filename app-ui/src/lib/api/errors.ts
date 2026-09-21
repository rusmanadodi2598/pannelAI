// Error type for every failed management call.
//
// The API message is preferred over the panel fallback, because it names the field or object the
// operator has to fix (docs/SPEC-UI/001-SPEC-UI.md §8.2). The fallback exists so a code with an
// empty message never reaches the screen as a blank banner.

import { fallbackMessage, schemaApiErrorEnvelope, type ApiErrorCode } from '$lib/schemas/error';

export type PanelErrorCode = ApiErrorCode | 'NETWORK' | 'DRIFT' | 'UNPARSABLE';

export class ApiError extends Error {
	constructor(
		readonly status: number,
		readonly code: PanelErrorCode,
		message: string,
		readonly requestId?: string
	) {
		super(message);
		this.name = 'ApiError';
	}
}

export function isRetryable(error: ApiError): boolean {
	return error.code === 'NETWORK' || error.code === 'INTERNAL_ERROR';
}

/**
 * Maps a failed response to the panel's error type.
 *
 * The body is passed in rather than read here, because one family of routes carries more than the §8
 * envelope on a refusal: the batch routes report every row by index (SPEC-API §8.1), and a `Response` body
 * can only be read once. The client reads it, maps the envelope with this function, and parses the rows
 * beside it.
 */
export function errorFromPayload(response: Response, payload: unknown): ApiError {
	const requestId = response.headers.get('x-request-id') ?? undefined;

	if (payload !== undefined) {
		const envelope = schemaApiErrorEnvelope.safeParse(payload);
		if (envelope.success) {
			const { code, message } = envelope.data.error;
			return new ApiError(
				response.status,
				code,
				message.trim() || fallbackMessage(code),
				requestId
			);
		}
	}

	return new ApiError(
		response.status,
		'UNPARSABLE',
		`The gateway returned ${response.status} without a readable error body.`,
		requestId
	);
}
