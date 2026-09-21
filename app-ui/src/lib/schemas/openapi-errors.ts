// The error code table, derived from the served v1 contract (docs/SPEC-UI/001-SPEC-UI.md §6.12).
//
// §6.12 asks for the error table from SPEC-API §8 without a second, hand-written copy of the contract.
// The document carries it: `x-contract.planes` names each plane's error envelope and maps every code to
// its status, and the meaning sentence is the description of the response whose body is that envelope.
//
// A code the document declares without describing says so in the cell rather than rendering blank,
// because a blank cell reads as a rendering fault instead of as a fact about the document.

import type { OpenAPIDocument } from './openapi';

/** What a code's meaning cell says when the document describes no response for it. */
export const NO_DESCRIPTION = 'No description in the document.';

export type ErrorCodeRow = { code: string; status: number; description?: string };
export type ErrorPlane = { key: string; label: string; envelope?: string; rows: ErrorCodeRow[] };

/** `management` to `Management`, `data_plane` to `Data plane`: the document's key, made readable. */
export function planeLabel(key: string): string {
	const words = key.split('_').filter((word) => word.length > 0);
	if (words.length === 0) return key;
	return [words[0][0].toUpperCase() + words[0].slice(1), ...words.slice(1)].join(' ');
}

/** One plane per entry in the document's `x-contract.planes`, with its codes ordered by status. */
export function errorPlanes(doc: OpenAPIDocument): ErrorPlane[] {
	const planes = doc['x-contract']?.planes ?? {};

	return Object.entries(planes).map(([key, plane]) => {
		const described = responseDescriptions(doc, plane.error_envelope);
		const rows = Object.entries(plane.codes ?? {})
			.map(([code, status]) => ({ code, status, description: described.get(code) }))
			.sort((left, right) => left.status - right.status || left.code.localeCompare(right.code));

		return { key, label: planeLabel(key), envelope: plane.error_envelope, rows };
	});
}

/**
 * The meaning sentences a plane's own responses carry. A response belongs to a plane when its JSON body
 * is the schema the plane names as its error envelope, which is what keeps the two planes' tables from
 * borrowing each other's wording.
 */
function responseDescriptions(doc: OpenAPIDocument, envelope?: string): Map<string, string> {
	const described = new Map<string, string>();
	if (!envelope) return described;

	const ref = `#/components/schemas/${envelope}`;

	for (const response of Object.values(doc.components?.responses ?? {})) {
		if (response.content?.['application/json']?.schema?.$ref !== ref) continue;

		for (const code of response['x-error-codes'] ?? []) {
			// First wins, so two responses declaring one code do not make the table depend on key order.
			if (response.description && !described.has(code)) described.set(code, response.description);
		}
	}

	return described;
}
