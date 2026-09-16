// Tests for the environment schema and the wire contracts the panel parses.

import { describe, expect, it } from 'vitest';
import { EnvError, parseEnv } from '$lib/schemas/env';
import { schemaApiErrorEnvelope } from '$lib/schemas/error';
import { schemaCreatedGatewayKey, schemaGatewayKey } from '$lib/schemas/gateway-key';

describe('parseEnv', () => {
	const cases = [
		{ name: 'a plain http target', input: { PANEL_API_TARGET: 'http://127.0.0.1:8080' }, ok: true },
		{
			name: 'an https target with a path',
			input: { PANEL_API_TARGET: 'https://gw.example.com' },
			ok: true
		},
		{ name: 'a trailing slash', input: { PANEL_API_TARGET: 'http://host:8080/' }, ok: true },
		{ name: 'empty', input: {}, ok: false },
		{ name: 'a relative value', input: { PANEL_API_TARGET: '/api/v1' }, ok: false },
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

	it('names the failing variable', () => {
		expect(() => parseEnv({ PANEL_API_TARGET: 'nope' })).toThrow(/PANEL_API_TARGET/);
	});

	it('strips a trailing slash so forwarding builds one path', () => {
		expect(parseEnv({ PANEL_API_TARGET: 'http://host:8080/' }).PANEL_API_TARGET).toBe(
			'http://host:8080'
		);
	});
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
		{ name: 'a non-object', input: 'boom', ok: false }
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
	it('tolerates an added field from the API', () => {
		expect(schemaGatewayKey.safeParse({ ...validKey, rate_limit_tier: 'pro' }).success).toBe(true);
	});
});

describe('created gateway key schema', () => {
	const cases = [
		{
			name: 'a create response',
			input: {
				id: validKey.id,
				name: 'Laptop',
				key: 'sk-live-abc',
				key_hint: 'sk-...abc',
				created_at: validKey.created_at
			},
			ok: true
		},
		{
			name: 'a response without the plaintext key',
			input: {
				id: validKey.id,
				name: 'Laptop',
				key: '',
				key_hint: 'sk-...abc',
				created_at: validKey.created_at
			},
			ok: false
		}
	];

	for (const testCase of cases) {
		it(`${testCase.ok ? 'accepts' : 'rejects'} ${testCase.name}`, () => {
			expect(schemaCreatedGatewayKey.safeParse(testCase.input).success).toBe(testCase.ok);
		});
	}
});
