// Catalog derivations from the served contract (docs/SPEC-UI/001-SPEC-UI.md §6.12).
//
// The transforms are what the screen is built from, so each is driven here with a table of variations
// (docs/RULLES/TDD.md §2.5) rather than with the fixture alone: the group order, the inherited
// credential, the untagged tail, and the multi-tag case are the ones a reader of the document will meet.

import { describe, expect, it } from 'vitest';
import {
	documentGroups,
	documentOperations,
	groupAnchor,
	UNTAGGED_GROUP
} from '$lib/schemas/openapi-catalog';
import { schemaOpenAPIDocument, type OpenAPIDocument } from '$lib/schemas/openapi';
import { openapiDocument } from '../support/openapi-document';

function parse(patch: Record<string, unknown> = {}): OpenAPIDocument {
	return schemaOpenAPIDocument.parse(openapiDocument(patch));
}

describe('documentOperations', () => {
	it('lists every operation in path order and in method order', () => {
		expect(
			documentOperations(parse()).map((operation) => `${operation.method} ${operation.path}`)
		).toEqual([
			'get /api/v1/health',
			'get /api/v1/gateway-keys',
			'post /api/v1/gateway-keys',
			'patch /api/v1/gateway-keys/{id}',
			'post /api/v1/chat/completions',
			'get /api/v1/settings'
		]);
	});

	it('reads a path-level parameters array as shared, not as an operation', () => {
		const doc = parse({
			paths: {
				'/api/v1/things/{id}': {
					parameters: [{ name: 'id', in: 'path' }],
					get: { summary: 'Read one' }
				}
			}
		});

		expect(documentOperations(doc).map((operation) => operation.method)).toEqual(['get']);
	});

	const credentialCases = [
		{
			name: 'an operation that declares no security inherits the document',
			path: '/api/v1/gateway-keys',
			method: 'get',
			expected: ['sessionCookie']
		},
		{
			name: 'an operation that declares none at all stays public',
			path: '/api/v1/health',
			method: 'get',
			expected: []
		},
		{
			name: 'an operation that overrides the document wins',
			path: '/api/v1/chat/completions',
			method: 'post',
			expected: ['gatewayKey']
		}
	];

	for (const testCase of credentialCases) {
		it(`resolves the credential when ${testCase.name}`, () => {
			const operation = documentOperations(parse()).find(
				(candidate) => candidate.path === testCase.path && candidate.method === testCase.method
			);

			expect(operation?.schemes).toEqual(testCase.expected);
		});
	}

	it('leaves an operation public when neither it nor the document declares a credential', () => {
		const doc = parse({ security: undefined, paths: { '/api/v1/x': { get: { tags: ['X'] } } } });

		expect(documentOperations(doc).every((operation) => operation.schemes.length === 0)).toBe(true);
	});
});

describe('documentGroups', () => {
	it('follows the document tag order, then the untagged tail', () => {
		expect(documentGroups(parse()).map((group) => group.name)).toEqual([
			'System',
			'Gateway Keys',
			'Data Plane',
			UNTAGGED_GROUP
		]);
	});

	it('drops a declared tag no operation uses', () => {
		const doc = parse({ tags: [{ name: 'System' }, { name: 'Nothing here' }] });

		expect(documentGroups(doc).map((group) => group.name)).toEqual([
			'System',
			'Gateway Keys',
			'Data Plane',
			UNTAGGED_GROUP
		]);
	});

	it('appends a tag only an operation declares', () => {
		const doc = parse({ tags: [], paths: { '/api/v1/x': { get: { tags: ['Later'] } } } });

		expect(documentGroups(doc).map((group) => group.name)).toEqual(['Later']);
	});

	it('lists an operation under every tag it declares', () => {
		const doc = parse({
			tags: [{ name: 'System' }, { name: 'Data Plane' }],
			paths: { '/api/v1/both': { get: { tags: ['System', 'Data Plane'] } } }
		});

		expect(documentGroups(doc).map((group) => [group.name, group.operations.length])).toEqual([
			['System', 1],
			['Data Plane', 1]
		]);
	});

	it('keeps the document order of operations inside a group', () => {
		const keys = documentGroups(parse())
			.find((group) => group.name === 'Gateway Keys')
			?.operations.map((operation) => `${operation.method} ${operation.path}`);

		expect(keys).toEqual([
			'get /api/v1/gateway-keys',
			'post /api/v1/gateway-keys',
			'patch /api/v1/gateway-keys/{id}'
		]);
	});
});

describe('groupAnchor', () => {
	const cases = [
		{ name: 'a single word', input: 'System', expected: 'group-system' },
		{ name: 'two words', input: 'Gateway Keys', expected: 'group-gateway-keys' },
		{ name: 'an acronym', input: 'API Docs', expected: 'group-api-docs' },
		{ name: 'punctuation', input: 'Media / Providers', expected: 'group-media-providers' },
		{ name: 'punctuation only', input: '...', expected: 'group' }
	];

	for (const testCase of cases) {
		it(`builds an anchor from ${testCase.name}`, () => {
			expect(groupAnchor(testCase.input)).toBe(testCase.expected);
		});
	}
});
