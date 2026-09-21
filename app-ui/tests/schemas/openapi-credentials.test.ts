// Credential labels, placements, and how many operations each scheme gates
// (docs/SPEC-UI/001-SPEC-UI.md §6.12).
//
// The labels are tables because a scheme the panel does not know must render as itself rather than as a
// guess, and the placement is derived from the scheme's own declaration rather than from the two names the
// panel happens to recognise today.

import { describe, expect, it } from 'vitest';
import {
	credentialHeader,
	credentialLabel,
	publicOperationCount,
	schemePlacement,
	schemeUses
} from '$lib/schemas/openapi-credentials';
import { schemaOpenAPIDocument, type OpenAPIDocument } from '$lib/schemas/openapi';
import { openapiDocument } from '../support/openapi-document';

function parse(patch: Record<string, unknown> = {}): OpenAPIDocument {
	return schemaOpenAPIDocument.parse(openapiDocument(patch));
}

describe('credentialLabel', () => {
	const cases = [
		{ name: 'no scheme', input: [], expected: 'No credential' },
		{ name: 'the session scheme', input: ['sessionCookie'], expected: 'Session cookie' },
		{ name: 'the gateway scheme', input: ['gatewayKey'], expected: 'Gateway key' },
		{ name: 'a scheme the panel does not know', input: ['mtls'], expected: 'mtls' },
		{
			name: 'two schemes',
			input: ['sessionCookie', 'gatewayKey'],
			expected: 'Session cookie or Gateway key'
		}
	];

	for (const testCase of cases) {
		it(`labels ${testCase.name}`, () => {
			expect(credentialLabel(testCase.input)).toBe(testCase.expected);
		});
	}
});

describe('schemePlacement', () => {
	const cases = [
		{
			name: 'an api key in a cookie',
			input: { type: 'apiKey', in: 'cookie', name: 'pannel_session' },
			expected: 'cookie pannel_session'
		},
		{
			name: 'an api key in a header',
			input: { type: 'apiKey', in: 'header', name: 'x-api-key' },
			expected: 'header x-api-key'
		},
		{ name: 'an api key with no name', input: { type: 'apiKey', in: 'query' }, expected: 'query' },
		{ name: 'an api key with no placement', input: { type: 'apiKey' }, expected: 'header' },
		{
			name: 'a bearer with a declared format',
			input: { type: 'http', scheme: 'bearer', bearerFormat: 'gateway-key' },
			expected: 'bearer (gateway-key)'
		},
		{
			name: 'an http scheme with no format',
			input: { type: 'http', scheme: 'basic' },
			expected: 'basic'
		},
		{
			name: 'a type the panel does not know',
			input: { type: 'openIdConnect' },
			expected: 'openIdConnect'
		}
	];

	for (const testCase of cases) {
		it(`describes ${testCase.name}`, () => {
			expect(schemePlacement(testCase.input)).toBe(testCase.expected);
		});
	}
});

describe('credentialHeader', () => {
	const cases = [
		{
			name: 'a cookie scheme',
			scheme: { type: 'apiKey', in: 'cookie', name: 'pannel_session' },
			expected: 'Cookie: pannel_session=<session cookie>'
		},
		{
			name: 'a bearer scheme',
			scheme: { type: 'http', scheme: 'bearer', bearerFormat: 'gateway-key' },
			expected: 'Authorization: Bearer sk-...'
		},
		{
			name: 'a basic scheme',
			scheme: { type: 'http', scheme: 'basic' },
			expected: 'Authorization: Basic <credential>'
		},
		{
			name: 'a header api key',
			scheme: { type: 'apiKey', in: 'header', name: 'x-key' },
			expected: 'x-key: <credential>'
		},
		{
			name: 'a scheme the document does not define',
			scheme: undefined,
			expected: 'mtls: <credential>'
		}
	];

	for (const testCase of cases) {
		it(`writes a placeholder for ${testCase.name}`, () => {
			expect(credentialHeader('mtls', testCase.scheme)).toBe(testCase.expected);
		});
	}
});

describe('schemeUses', () => {
	it('counts the operations each scheme gates', () => {
		expect(schemeUses(parse()).map((use) => [use.name, use.operations])).toEqual([
			['sessionCookie', 4],
			['gatewayKey', 1]
		]);
	});

	it('keeps a defined scheme that no operation uses, with a count of zero', () => {
		const doc = parse({
			components: {
				securitySchemes: { legacy: { type: 'apiKey', in: 'header', name: 'x-legacy' } }
			}
		});

		expect(schemeUses(doc)).toContainEqual({
			name: 'legacy',
			scheme: { type: 'apiKey', in: 'header', name: 'x-legacy' },
			operations: 0
		});
		expect(schemeUses(doc).map((use) => [use.name, use.operations])).toContainEqual([
			'sessionCookie',
			4
		]);
	});

	it('names a scheme an operation uses but the document never defines', () => {
		const named = schemeUses(parse({ security: [{ mtls: [] }] })).find(
			(use) => use.name === 'mtls'
		);

		// The four operations that inherit the document's credential name a scheme it never defines.
		expect(named?.scheme).toBeUndefined();
		expect(named?.operations).toBe(4);
	});
});

describe('publicOperationCount', () => {
	const cases = [
		{ name: 'the fixture, which holds one public operation', patch: {}, expected: 1 },
		{
			name: 'a document that gates everything',
			patch: { paths: { '/api/v1/x': { get: { tags: ['X'] } } } },
			expected: 0
		},
		{ name: 'a document with no security at all', patch: { security: undefined }, expected: 5 }
	];

	for (const testCase of cases) {
		it(`counts ${testCase.name}`, () => {
			expect(publicOperationCount(parse(testCase.patch))).toBe(testCase.expected);
		});
	}
});
