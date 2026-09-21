// The error code table derived from the served contract (docs/SPEC-UI/001-SPEC-UI.md §6.12).
//
// The table is §6.12's answer to "the error code table from SPEC-API §8 without a second copy": the codes
// come from the document, and each meaning comes from the response that carries that plane's envelope.
// The cases below are the ones that make that rule checkable: two planes sharing a code name, a code the
// document declares without describing, and a document that carries no contract block at all.

import { describe, expect, it } from 'vitest';
import { errorPlanes, planeLabel } from '$lib/schemas/openapi-errors';
import { schemaOpenAPIDocument, type OpenAPIDocument } from '$lib/schemas/openapi';
import { openapiDocument } from '../support/openapi-document';

function parse(patch: Record<string, unknown> = {}): OpenAPIDocument {
	return schemaOpenAPIDocument.parse(openapiDocument(patch));
}

describe('errorPlanes', () => {
	it('reads one table per plane, ordered by status', () => {
		expect(
			errorPlanes(parse()).map((plane) => [
				plane.label,
				plane.envelope,
				plane.rows.map((row) => row.code)
			])
		).toEqual([
			['Management', 'ManagementError', ['VALIDATION_ERROR', 'UNAUTHORIZED']],
			['Data plane', 'DataPlaneError', ['MODEL_NOT_FOUND', 'UNAUTHORIZED']]
		]);
	});

	it('takes each meaning from its own plane, so one code can read two ways', () => {
		const planes = errorPlanes(parse());
		const management = planes[0].rows.find((row) => row.code === 'UNAUTHORIZED');
		const dataPlane = planes[1].rows.find((row) => row.code === 'UNAUTHORIZED');

		expect(management?.description).toBe('No valid session cookie was presented.');
		expect(dataPlane?.description).toBe('The gateway key was missing or invalid.');
	});

	it('leaves a code the document declares without describing it', () => {
		const row = errorPlanes(parse())[1].rows.find(
			(candidate) => candidate.code === 'MODEL_NOT_FOUND'
		);

		expect(row?.status).toBe(400);
		expect(row?.description).toBeUndefined();
	});

	it('reports no plane when the document carries no contract block', () => {
		expect(errorPlanes(parse({ 'x-contract': undefined }))).toEqual([]);
	});

	it('keeps the first description when two responses declare one code', () => {
		const doc = parse({
			components: {
				responses: {
					ManagementUnauthorizedError: {
						description: 'First sentence.',
						'x-error-codes': ['UNAUTHORIZED'],
						content: {
							'application/json': { schema: { $ref: '#/components/schemas/ManagementError' } }
						}
					},
					ManagementUnauthorizedAgain: {
						description: 'Second sentence.',
						'x-error-codes': ['UNAUTHORIZED'],
						content: {
							'application/json': { schema: { $ref: '#/components/schemas/ManagementError' } }
						}
					}
				}
			}
		});

		const row = errorPlanes(doc)[0].rows.find((candidate) => candidate.code === 'UNAUTHORIZED');
		expect(row?.description).toBe('First sentence.');
	});
});

describe('planeLabel', () => {
	const cases = [
		{ name: 'a single word', input: 'management', expected: 'Management' },
		{ name: 'an underscored pair', input: 'data_plane', expected: 'Data plane' },
		{ name: 'an empty key', input: '', expected: '' }
	];

	for (const testCase of cases) {
		it(`labels ${testCase.name}`, () => {
			expect(planeLabel(testCase.input)).toBe(testCase.expected);
		});
	}
});
