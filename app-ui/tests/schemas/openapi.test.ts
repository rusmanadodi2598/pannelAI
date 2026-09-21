// The served contract's schema (docs/SPEC-UI/001-SPEC-UI.md §6.12, §7.4).
//
// Three tables, per docs/RULLES/TDD.md §2.5. The rejections matter as much as the acceptances: this is
// the drift gate for the one wire shape the panel did not write, and a document that lost `paths` or
// broke an operation must be reported rather than rendered as an empty catalog.

import { describe, expect, it } from 'vitest';
import { schemaOpenAPIDocument } from '$lib/schemas/openapi';
import { openapiDocument } from '../support/openapi-document';

describe('openapi document schema', () => {
	const accepted = [
		{ name: 'the fixture', input: openapiDocument() },
		{
			name: 'a document with paths and info only',
			input: { openapi: '3.1.0', info: { title: 'x', version: 'v0' }, paths: {} }
		},
		{
			name: 'a document whose server list is empty',
			input: openapiDocument({ servers: [] })
		},
		{
			name: 'a document with no tags, no components, and no x-contract',
			input: openapiDocument({ tags: undefined, components: undefined, 'x-contract': undefined })
		},
		{
			name: 'a path item carrying path-level parameters',
			input: openapiDocument({
				paths: {
					'/api/v1/things/{id}': {
						parameters: [{ name: 'id', in: 'path', required: true }],
						get: { summary: 'Read one' }
					}
				}
			})
		}
	];

	for (const testCase of accepted) {
		it(`accepts ${testCase.name}`, () => {
			expect(schemaOpenAPIDocument.safeParse(testCase.input).success).toBe(true);
		});
	}

	const rejected = [
		{ name: 'a null body', input: null },
		{ name: 'a string body', input: 'contract' },
		{
			name: 'a document with no paths',
			input: { openapi: '3.1.0', info: { title: 'x', version: 'v0' } }
		},
		{
			name: 'a document with no version',
			input: { openapi: '3.1.0', info: { title: 'x' }, paths: {} }
		},
		{
			name: 'an operation that is a string',
			input: openapiDocument({ paths: { '/api/v1/x': { get: 'read it' } } })
		},
		{
			name: 'a security requirement whose value is not a list',
			input: openapiDocument({ security: [{ sessionCookie: 'all' }] })
		}
	];

	for (const testCase of rejected) {
		it(`rejects ${testCase.name}`, () => {
			expect(schemaOpenAPIDocument.safeParse(testCase.input).success).toBe(false);
		});
	}

	// Additive change must not break the panel (§7.4.2): the document belongs to the gateway.
	const additive = [
		{ name: 'a new top-level block', patch: { 'x-generated-at': '2026-09-21' } },
		{
			name: 'a new operation key',
			patch: { paths: { '/api/v1/health': { get: { summary: 'Liveness', deprecated: true } } } }
		},
		{
			name: 'a new path-level key',
			patch: {
				paths: { '/api/v1/health': { summary: 'System routes', get: { summary: 'Liveness' } } }
			}
		},
		{
			name: 'a new scheme property',
			patch: {
				components: {
					securitySchemes: {
						sessionCookie: {
							type: 'apiKey',
							in: 'cookie',
							name: 'pannel_session',
							deprecated: false
						}
					}
				}
			}
		}
	];

	for (const testCase of additive) {
		it(`tolerates ${testCase.name}`, () => {
			expect(schemaOpenAPIDocument.safeParse(openapiDocument(testCase.patch)).success).toBe(true);
		});
	}
});
