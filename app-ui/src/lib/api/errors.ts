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

export async function errorFromResponse(response: Response): Promise<ApiError> {
	const requestId = response.headers.get('x-request-id') ?? undefined;
	const raw = await readJson(response);

	if (raw !== undefined) {
		const envelope = schemaApiErrorEnvelope.safeParse(raw);
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

async function readJson(response: Response): Promise<unknown> {
	try {
		return await response.json();
	} catch {
		return undefined;
	}
}
