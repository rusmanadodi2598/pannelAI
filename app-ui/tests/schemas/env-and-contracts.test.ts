// Tests for the environment schema and the wire contracts the panel parses.
//
// Each group is a table of input variations, per docs/RULLES/TDD.md §2.5. The environment rows cover
// the failure modes a deployment actually hits: a missing variable, a malformed one, a typo that
// starts with the PANEL_ prefix, and a variable that belongs to something else.

import { describe, expect, it } from 'vitest';
import { EnvError, parseEnv } from '$lib/schemas/env';
import { schemaApiErrorEnvelope } from '$lib/schemas/error';
import { schemaCreatedGatewayKey, schemaGatewayKey } from '$lib/schemas/gateway-key';

describe('parseEnv', () => {
	const cases = [
		{ name: 'a plain http target', input: { PANEL_API_TARGET: 'http://127.0.0.1:8080' }, ok: true },
		{
			name: 'an https target with a path',
			input: { PANEL_API_TARGET: 'https://gw.example.com/v1' },
			ok: true
		},
		{ name: 'a trailing slash', input: { PANEL_API_TARGET: 'http://host:8080/' }, ok: true },
		{
			name: 'a variable that is not ours',
			input: { PANEL_API_TARGET: 'http://host', PATH: '/usr/bin' },
			ok: true
		},
		{ name: 'empty', input: {}, ok: false },
		{ name: 'a relative value', input: { PANEL_API_TARGET: '/api/v1' }, ok: false },
		{ name: 'a non-http scheme', input: { PANEL_API_TARGET: 'ftp://host' }, ok: false },
		{
			name: 'an unknown panel variable',
			input: { PANEL_API_TARGET: 'http://h', PANEL_TYPO: 'x' },
			ok: false
		}
	];

	for (const testCase of cases) {
		it(`${testCase.ok ? 'accepts' : 'rejects'} ${testCase.name}`, () => {
			if (testCase.ok) {
				expect(() => parseEnv(testCase.input)).not.toThrow();
				return;
			}

			expect(() => parseEnv(testCase.input)).toThrow(EnvError);
		});
	}

	const failureCases = [
		{ name: 'a missing variable', input: {}, pattern: /PANEL_API_TARGET/ },
		{ name: 'a malformed value', input: { PANEL_API_TARGET: 'nope' }, pattern: /PANEL_API_TARGET/ },
		{
			name: 'a relative target',
			input: { PANEL_API_TARGET: '/api/v1' },
			pattern: /PANEL_API_TARGET/
		},
		{
			name: 'an unknown panel variable',
			input: { PANEL_API_TARGET: 'http://h', PANEL_TYPO: 'x' },
			pattern: /PANEL_TYPO/
		}
	];

	for (const testCase of failureCases) {
		it(`names the failing variable for ${testCase.name}`, () => {
			expect(() => parseEnv(testCase.input)).toThrow(testCase.pattern);
		});
	}

	const normalizationCases = [
		{ name: 'a trailing slash', input: 'http://host:8080/', expected: 'http://host:8080' },
		{
			name: 'a path with a trailing slash',
			input: 'https://gw.example.com/v1/',
			expected: 'https://gw.example.com/v1'
		},
		{ name: 'no trailing slash', input: 'http://host:8080', expected: 'http://host:8080' },
		{ name: 'surrounding whitespace', input: '  http://host:8080  ', expected: 'http://host:8080' }
	];

	for (const testCase of normalizationCases) {
		it(`normalizes ${testCase.name} so forwarding builds one path`, () => {
			expect(parseEnv({ PANEL_API_TARGET: testCase.input }).PANEL_API_TARGET).toBe(
				testCase.expected
			);
		});
	}
});

describe('error envelope', () => {
	const cases = [
		{ name: 'a known code', input: { error: { code: 'NOT_FOUND', message: 'gone' } }, ok: true },
		{
			name: 'a rate limit code',
			input: { error: { code: 'RATE_LIMITED', message: '' } },
			ok: true
		},
		{ name: 'an unknown code', input: { error: { code: 'TEAPOT', message: 'x' } }, ok: false },
		{ name: 'a missing error object', input: { message: 'x' }, ok: false },
		{ name: 'a non-object', input: 'boom', ok: false },
		{ name: 'a null body', input: null, ok: false }
	];

	for (const testCase of cases) {
		it(`${testCase.ok ? 'accepts' : 'rejects'} ${testCase.name}`, () => {
			expect(schemaApiErrorEnvelope.safeParse(testCase.input).success).toBe(testCase.ok);
		});
	}
});

const validKey = {
	id: 'gky_01HZY0000000000000000000AB',
	name: 'Laptop',
	key_hint: 'sk-...abcd',
	status: 'active',
	last_used_at: null,
	request_count: 12,
	created_at: '2026-09-16T00:00:00Z',
	revoked_at: null
};

describe('gateway key schema', () => {
	const cases = [
		{ name: 'a complete row', input: validKey, ok: true },
		{ name: 'a row with no last use', input: { ...validKey, last_used_at: null }, ok: true },
		{ name: 'a zero request count', input: { ...validKey, request_count: 0 }, ok: true },
		{ name: 'a wrong id prefix', input: { ...validKey, id: 'ep_01HZY' }, ok: false },
		{ name: 'a negative request count', input: { ...validKey, request_count: -1 }, ok: false },
		{ name: 'a fractional request count', input: { ...validKey, request_count: 1.5 }, ok: false },
		{ name: 'a broken timestamp', input: { ...validKey, created_at: 'yesterday' }, ok: false },
		{ name: 'a missing name', input: { ...validKey, name: undefined }, ok: false }
	];

	for (const testCase of cases) {
		it(`${testCase.ok ? 'accepts' : 'rejects'} ${testCase.name}`, () => {
			expect(schemaGatewayKey.safeParse(testCase.input).success).toBe(testCase.ok);
		});
	}

	// Additive fields must not break the panel, which is the whole reason response schemas are not
	// strict (docs/SPEC-UI/001-SPEC-UI.md §7.4.2).
	const additiveCases = [
		{ name: 'a new string field', patch: { rate_limit_tier: 'pro' } },
		{ name: 'a new numeric field', patch: { request_count_7d: 3 } },
		{ name: 'a new object field', patch: { limits: { rpm: 60 } } },
		{ name: 'a new null field', patch: { deleted_at: null } }
	];

	for (const testCase of additiveCases) {
		it(`tolerates ${testCase.name} from the API`, () => {
			expect(schemaGatewayKey.safeParse({ ...validKey, ...testCase.patch }).success).toBe(true);
		});
	}
});

describe('created gateway key schema', () => {
	// The field name is the served one. `POST /api/v1/gateway-keys` answers with `plaintext_key` (and
	// only on that route); this fixture used to say `key`, which is the name the panel had guessed.
	const validCreate = {
		id: validKey.id,
		name: 'Laptop',
		plaintext_key: 'sk-live-abc',
		key_hint: 'sk-...abc',
		created_at: validKey.created_at
	};

	const cases = [
		{ name: 'a create response', input: validCreate, ok: true },
		{
			name: 'a response without the plaintext key',
			input: { ...validCreate, plaintext_key: '' },
			ok: false
		},
		{
			name: 'a response that names the plaintext field `key`',
			input: { ...validCreate, key: 'sk-live-abc', plaintext_key: undefined },
			ok: false
		},
		{
			name: 'a response with a wrong id prefix',
			input: { ...validCreate, id: 'ep_01HZY' },
			ok: false
		},
		{
			name: 'a response with a broken timestamp',
			input: { ...validCreate, created_at: 'today' },
			ok: false
		},
		{ name: 'a response with no name', input: { ...validCreate, name: undefined }, ok: false }
	];

	for (const testCase of cases) {
		it(`${testCase.ok ? 'accepts' : 'rejects'} ${testCase.name}`, () => {
			expect(schemaCreatedGatewayKey.safeParse(testCase.input).success).toBe(testCase.ok);
		});
	}
});
