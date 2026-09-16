// Management error envelope, mirroring docs/SPEC-API/001-SPEC-API.md §8.
//
// The code list is a closed enum because the API spec defines it as closed. A code the panel does
// not know is a contract change, and the panel reports it rather than rendering a blank message.

import { z } from 'zod';

export const API_ERROR_CODES = [
	'VALIDATION_ERROR',
	'UNAUTHORIZED',
	'FORBIDDEN',
	'NOT_FOUND',
	'METHOD_NOT_ALLOWED',
	'CONFLICT',
	'RATE_LIMITED',
	'NO_PROVIDER_AVAILABLE',
	'UPSTREAM_ERROR',
	'UPSTREAM_TIMEOUT',
	'INTERNAL_ERROR'
] as const;

export const apiErrorCode = z.enum(API_ERROR_CODES);
export type ApiErrorCode = z.infer<typeof apiErrorCode>;

export const schemaApiErrorEnvelope = z.object({
	error: z.object({
		code: apiErrorCode,
		message: z.string()
	})
});

export type ApiErrorEnvelope = z.infer<typeof schemaApiErrorEnvelope>;

// One English line per code, used when the API message is missing or empty. The API message is
// preferred when present, because it names the field or object the operator needs to fix (§8.2).
export const API_ERROR_FALLBACK: Record<ApiErrorCode, string> = {
	VALIDATION_ERROR: 'The gateway rejected these values. Check the highlighted fields.',
	UNAUTHORIZED: 'Your session ended. Sign in again to continue.',
	FORBIDDEN: 'This action is not permitted for the current session.',
	NOT_FOUND: 'That item no longer exists. Go back to the list.',
	METHOD_NOT_ALLOWED: 'The gateway does not allow that action on this address.',
	CONFLICT: 'The gateway refused this change because something else depends on it.',
	RATE_LIMITED: 'Too many attempts. Wait a moment, then try again.',
	NO_PROVIDER_AVAILABLE: 'No upstream endpoint is healthy for that model right now.',
	UPSTREAM_ERROR: 'The upstream provider returned an error.',
	UPSTREAM_TIMEOUT: 'The upstream provider did not answer in time.',
	INTERNAL_ERROR: 'The gateway hit an unexpected error. Quote the request ID when reporting it.'
};

export function fallbackMessage(code: ApiErrorCode): string {
	return API_ERROR_FALLBACK[code];
}
