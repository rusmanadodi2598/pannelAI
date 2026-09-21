// The example call, and the operation a group shows one for (docs/SPEC-UI/001-SPEC-UI.md §6.12).
//
// This is the one part of the screen that composes a string a reader may paste into a shell, so every
// branch is checked: the credential line, the path placeholder, a document that declares no server, and
// the promise that no real key can appear in it.

import { describe, expect, it } from 'vitest';
import { curlExample, documentBaseUrl, examplePath } from '$lib/schemas/openapi-credentials';
import { documentGroups, groupExample, type ApiOperation } from '$lib/schemas/openapi-catalog';
import { schemaOpenAPIDocument, type OpenAPIDocument } from '$lib/schemas/openapi';
import { openapiDocument } from '../support/openapi-document';

function parse(patch: Record<string, unknown> = {}): OpenAPIDocument {
	return schemaOpenAPIDocument.parse(openapiDocument(patch));
}

function operation(
	doc: OpenAPIDocument,
	path: string,
	method: ApiOperation['method']
): ApiOperation {
	const found = documentGroups(doc)
		.flatMap((group) => group.operations)
		.find((candidate) => candidate.path === path && candidate.method === method);

	if (!found) throw new Error(`the fixture has no ${method} ${path}`);
	return found;
}

describe('examplePath', () => {
	const cases = [
		{ name: 'a plain path', input: '/api/v1/health', expected: '/api/v1/health' },
		{
			name: 'one parameter',
			input: '/api/v1/gateway-keys/{id}',
			expected: '/api/v1/gateway-keys/<id>'
		},
		{
			name: 'two parameters',
			input: '/api/v1/providers/{provider_id}/keys/{key_id}',
			expected: '/api/v1/providers/<provider_id>/keys/<key_id>'
		}
	];

	for (const testCase of cases) {
		it(`replaces the braces in ${testCase.name}`, () => {
			expect(examplePath(testCase.input)).toBe(testCase.expected);
		});
	}
});

describe('documentBaseUrl', () => {
	const cases = [
		{ name: 'a declared server', patch: {}, expected: 'http://localhost:8080' },
		{
			name: 'a trailing slash',
			patch: { servers: [{ url: 'http://h:8080/' }] },
			expected: 'http://h:8080'
		},
		{ name: 'no server at all', patch: { servers: undefined }, expected: '<gateway base URL>' },
		{ name: 'an empty server list', patch: { servers: [] }, expected: '<gateway base URL>' }
	];

	for (const testCase of cases) {
		it(`reads ${testCase.name}`, () => {
			expect(documentBaseUrl(parse(testCase.patch))).toBe(testCase.expected);
		});
	}
});

describe('curlExample', () => {
	const cases = [
		{
			name: 'a session-gated read',
			path: '/api/v1/gateway-keys',
			method: 'get' as const,
			expected: `curl -X GET 'http://localhost:8080/api/v1/gateway-keys' \\
  -H 'Cookie: pannel_session=<session cookie>'`
		},
		{
			name: 'a gateway-key call',
			path: '/api/v1/chat/completions',
			method: 'post' as const,
			expected: `curl -X POST 'http://localhost:8080/api/v1/chat/completions' \\
  -H 'Authorization: Bearer sk-...'`
		},
		{
			name: 'a call with a path parameter',
			path: '/api/v1/gateway-keys/{id}',
			method: 'patch' as const,
			expected: `curl -X PATCH 'http://localhost:8080/api/v1/gateway-keys/<id>' \\
  -H 'Cookie: pannel_session=<session cookie>'`
		},
		{
			name: 'a public call, which carries no credential line',
			path: '/api/v1/health',
			method: 'get' as const,
			expected: `curl -X GET 'http://localhost:8080/api/v1/health'`
		}
	];

	for (const testCase of cases) {
		it(`composes ${testCase.name}`, () => {
			const doc = parse();
			expect(curlExample(operation(doc, testCase.path, testCase.method), doc)).toBe(
				testCase.expected
			);
		});
	}

	it('states a placeholder when the document declares no server', () => {
		const doc = parse({ servers: undefined });

		expect(curlExample(operation(doc, '/api/v1/health', 'get'), doc)).toBe(
			`curl -X GET '<gateway base URL>/api/v1/health'`
		);
	});

	it('falls back to the scheme name when the document does not define it', () => {
		const doc = parse({ security: [{ mtls: [] }], components: { securitySchemes: {} } });

		expect(curlExample(operation(doc, '/api/v1/gateway-keys', 'get'), doc)).toContain(
			"-H 'mtls: <credential>'"
		);
	});

	it('carries no key material, only the placeholder §6.12 requires', () => {
		const doc = parse();
		const gatewayCall = curlExample(operation(doc, '/api/v1/chat/completions', 'post'), doc);

		expect(gatewayCall).toContain('sk-...');
		// A real gateway key is `sk-` followed by something other than the three dots.
		expect(gatewayCall).not.toMatch(/sk-(?!\.\.\.)[A-Za-z0-9]/);
	});
});

describe('groupExample', () => {
	it('prefers the group first read, because a GET runs as printed', () => {
		const group = documentGroups(parse()).find((candidate) => candidate.name === 'Gateway Keys');

		expect(groupExample(group!)?.method).toBe('get');
	});

	it('falls back to the first operation when the group has no read', () => {
		const doc = parse({ paths: { '/api/v1/x': { post: { tags: ['Only'] } } } });

		expect(groupExample(documentGroups(doc)[0])?.method).toBe('post');
	});
});
