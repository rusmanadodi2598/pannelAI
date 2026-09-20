// Tests for the alias contracts (docs/SPEC-UI/001-SPEC-UI.md §6.3, docs/SPEC-API/001-SPEC-API.md §7.6).
//
// Each group is a table of input variations, per docs/RULLES/TDD.md §2.5. The rules under test are
// app-serv's own, read from `validateCatalogText`, `NewModelAlias`, and `validateComboRef`, so the tables
// carry both fields' boundaries and the shapes the domain refuses.

import { describe, expect, it } from 'vitest';
import {
	aliasName,
	aliasSetBody,
	aliasTarget,
	schemaAliasEntry,
	schemaAliasSet,
	schemaReplaceAliasesBody,
	withAlias,
	withoutAlias
} from '$lib/schemas/model-alias';

describe('aliasName', () => {
	const cases = [
		{ name: 'a plain name', input: 'fast', ok: true },
		{ name: 'surrounding whitespace', input: '  fast  ', ok: true },
		{ name: 'dots, dashes, and underscores', input: 'gpt-4o.mini_fast', ok: true },
		{ name: 'the 120 character boundary', input: 'a'.repeat(120), ok: true },
		{ name: 'one character past the boundary', input: 'a'.repeat(121), ok: false },
		{ name: 'empty', input: '', ok: false },
		{ name: 'whitespace only', input: '   ', ok: false },
		{ name: 'a slash, which would read as provider/model', input: 'openai/gpt-4o', ok: false },
		{ name: 'a control character', input: 'fa\u0000st', ok: false },
		// The API's `validateCatalogText` allows an internal space, so the panel does too: refusing it here
		// would refuse a write the gateway accepts.
		{ name: 'an internal space', input: 'my alias', ok: true }
	];

	for (const testCase of cases) {
		it(`${testCase.ok ? 'accepts' : 'refuses'} ${testCase.name}`, () => {
			const parsed = aliasName.safeParse(testCase.input);

			expect(parsed.success).toBe(testCase.ok);
			if (parsed.success) expect(parsed.data).toBe(testCase.input.trim());
		});
	}
});

describe('aliasTarget', () => {
	const cases = [
		{ name: 'a provider/model reference', input: 'openai/gpt-4o', ok: true },
		{ name: 'surrounding whitespace', input: ' openai/gpt-4o ', ok: true },
		{ name: 'a combo name', input: 'fallback-combo', ok: true },
		// `ParseModelRef` splits at the first slash, so the model half may hold one.
		{ name: 'a reference whose model half holds a slash', input: 'openai/gpt/4o', ok: true },
		{ name: 'the 200 character boundary', input: `a/${'b'.repeat(198)}`, ok: true },
		{ name: 'one character past the boundary', input: `a/${'b'.repeat(199)}`, ok: false },
		{ name: 'empty', input: '', ok: false },
		{ name: 'an internal space', input: 'openai/gpt 4o', ok: false },
		{ name: 'a trailing slash', input: 'openai/', ok: false },
		{ name: 'a leading slash', input: '/gpt-4o', ok: false },
		{ name: 'a character a combo name may not hold', input: 'fast$name', ok: false }
	];

	for (const testCase of cases) {
		it(`${testCase.ok ? 'accepts' : 'refuses'} ${testCase.name}`, () => {
			const parsed = aliasTarget.safeParse(testCase.input);

			expect(parsed.success).toBe(testCase.ok);
			if (parsed.success) expect(parsed.data).toBe(testCase.input.trim());
		});
	}
});

describe('withAlias', () => {
	const cases = [
		{
			name: 'an empty set',
			entries: [],
			entry: { alias: 'fast', target: 'openai/gpt-4o' },
			want: [{ alias: 'fast', target: 'openai/gpt-4o' }]
		},
		{
			name: 'a set that already holds another alias',
			entries: [{ alias: 'smart', target: 'anthropic/claude' }],
			entry: { alias: 'fast', target: 'openai/gpt-4o' },
			want: [
				{ alias: 'smart', target: 'anthropic/claude' },
				{ alias: 'fast', target: 'openai/gpt-4o' }
			]
		},
		{
			// The primary key is the alias, so a second row with the same name cannot be stored: the merge
			// changes the target instead of producing a duplicate.
			name: 'a name that is already there, which changes its target',
			entries: [
				{ alias: 'fast', target: 'anthropic/claude' },
				{ alias: 'smart', target: 'openai/gpt-4o' }
			],
			entry: { alias: 'fast', target: 'openai/gpt-4o-mini' },
			want: [
				{ alias: 'fast', target: 'openai/gpt-4o-mini' },
				{ alias: 'smart', target: 'openai/gpt-4o' }
			]
		},
		{
			name: 'untrimmed fields on both sides',
			entries: [{ alias: ' smart ', target: ' anthropic/claude ' }],
			entry: { alias: ' fast ', target: ' openai/gpt-4o ' },
			want: [
				{ alias: 'smart', target: 'anthropic/claude' },
				{ alias: 'fast', target: 'openai/gpt-4o' }
			]
		}
	];

	for (const testCase of cases) {
		it(`adds ${testCase.name}`, () => {
			expect(withAlias(testCase.entries, testCase.entry)).toEqual(testCase.want);
		});
	}
});

describe('withoutAlias', () => {
	const entries = [
		{ alias: 'fast', target: 'openai/gpt-4o' },
		{ alias: 'smart', target: 'anthropic/claude' }
	];
	const cases = [
		{
			name: 'the first alias',
			input: 'fast',
			want: [{ alias: 'smart', target: 'anthropic/claude' }]
		},
		{
			name: 'the last alias',
			input: 'smart',
			want: [{ alias: 'fast', target: 'openai/gpt-4o' }]
		},
		{
			name: 'an untrimmed name',
			input: ' fast ',
			want: [{ alias: 'smart', target: 'anthropic/claude' }]
		},
		{ name: 'a name that is not there', input: 'ghost', want: entries },
		{ name: 'an empty name', input: '', want: entries }
	];

	for (const testCase of cases) {
		it(`removes ${testCase.name} and carries the rest`, () => {
			expect(withoutAlias(entries, testCase.input)).toEqual(testCase.want);
		});
	}

	it('removes every alias from a set of one', () => {
		expect(withoutAlias([{ alias: 'fast', target: 'openai/gpt-4o' }], 'fast')).toEqual([]);
	});
});

describe('aliasSetBody', () => {
	it('sorts by alias and trims both fields, because the answer echoes this order', () => {
		const body = aliasSetBody([
			{ alias: ' zeta ', target: ' openai/gpt-4o ' },
			{ alias: 'alpha', target: 'fallback-combo' },
			{ alias: 'Mid', target: 'openai/gpt-4o-mini' }
		]);

		expect(body).toEqual({
			aliases: [
				{ alias: 'Mid', target: 'openai/gpt-4o-mini' },
				{ alias: 'alpha', target: 'fallback-combo' },
				{ alias: 'zeta', target: 'openai/gpt-4o' }
			]
		});
	});

	it('carries an empty set through, which is how every alias is cleared', () => {
		expect(aliasSetBody([])).toEqual({ aliases: [] });
	});
});

describe('alias write shapes', () => {
	it('refuses a field the API does not read', () => {
		const parsed = schemaAliasEntry.safeParse({
			alias: 'fast',
			target: 'openai/gpt-4o',
			note: 'x'
		});

		expect(parsed.success).toBe(false);
	});

	it('refuses an entry inside the body that breaks a field rule', () => {
		const parsed = schemaReplaceAliasesBody.safeParse({
			aliases: [{ alias: 'fast', target: 'openai/' }]
		});

		expect(parsed.success).toBe(false);
	});

	it('accepts an empty body, which clears the set', () => {
		expect(schemaReplaceAliasesBody.safeParse({ aliases: [] }).success).toBe(true);
	});

	// §7.4.2: a read tolerates fields the panel does not know about, so an added one is not a failure.
	it('reads a set with an unknown field', () => {
		const parsed = schemaAliasSet.safeParse({
			data: [{ alias: 'fast', target: 'openai/gpt-4o', created_at: '2026-09-20T03:00:00Z' }]
		});

		expect(parsed.success).toBe(true);
		expect(parsed.success && parsed.data.data[0]).toEqual({
			alias: 'fast',
			target: 'openai/gpt-4o'
		});
	});

	it('refuses a read entry with no target', () => {
		expect(schemaAliasSet.safeParse({ data: [{ alias: 'fast' }] }).success).toBe(false);
	});
});
