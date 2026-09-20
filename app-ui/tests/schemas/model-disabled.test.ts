// Tests for the disabled set operations (docs/SPEC-UI/001-SPEC-UI.md §6.3).
//
// The operations decide whether a write erases another provider's rows, so they are tested as set
// algebra rather than as fixtures: every case varies the input set and the ref, and asserts the whole
// resulting set rather than a length.

import { describe, expect, it } from 'vitest';
import {
	disabledRefKey,
	providerDisabledRefs,
	schemaDisabledSet,
	schemaReplaceDisabledBody,
	withDisabledRef,
	withoutDisabledRef,
	type DisabledRef
} from '$lib/schemas/model-disabled';

const ref = (provider_id: string, model_id: string): DisabledRef => ({ provider_id, model_id });

describe('disabledRefKey', () => {
	it.each([
		{ ref: ref('openai', 'gpt-4o'), expected: 'openai/gpt-4o' },
		{ ref: ref('anthropic', 'claude-3-5-sonnet'), expected: 'anthropic/claude-3-5-sonnet' },
		{ ref: ref('a', 'b'), expected: 'a/b' },
		{ ref: ref('openai', 'gpt-4o-mini'), expected: 'openai/gpt-4o-mini' },
		{ ref: ref('local', 'llama3.1:8b'), expected: 'local/llama3.1:8b' }
	])('keys $ref as $expected', ({ ref: entry, expected }) => {
		expect(disabledRefKey(entry)).toBe(expected);
	});
});

describe('withDisabledRef', () => {
	it.each([
		{
			label: 'into an empty set',
			refs: [],
			ref: ref('openai', 'gpt-4o'),
			expected: [ref('openai', 'gpt-4o')]
		},
		{
			label: 'alongside another provider, which stays',
			refs: [ref('anthropic', 'claude-3-5-sonnet')],
			ref: ref('openai', 'gpt-4o'),
			expected: [ref('anthropic', 'claude-3-5-sonnet'), ref('openai', 'gpt-4o')]
		},
		{
			label: 'a ref the set already holds, without duplicating it',
			refs: [ref('openai', 'gpt-4o'), ref('anthropic', 'claude-3-5-sonnet')],
			ref: ref('openai', 'gpt-4o'),
			expected: [ref('anthropic', 'claude-3-5-sonnet'), ref('openai', 'gpt-4o')]
		},
		{
			label: 'into a set that came back duplicated',
			refs: [ref('openai', 'gpt-4o'), ref('openai', 'gpt-4o')],
			ref: ref('openai', 'gpt-4o-mini'),
			expected: [ref('openai', 'gpt-4o'), ref('openai', 'gpt-4o-mini')]
		},
		{
			label: 'ordered by provider then model, not by insertion',
			refs: [ref('openai', 'gpt-4o'), ref('anthropic', 'claude-3-5-sonnet')],
			ref: ref('anthropic', 'claude-3-haiku'),
			expected: [
				ref('anthropic', 'claude-3-5-sonnet'),
				ref('anthropic', 'claude-3-haiku'),
				ref('openai', 'gpt-4o')
			]
		}
	])('adds $label', ({ refs, ref: entry, expected }) => {
		expect(withDisabledRef(refs, entry)).toEqual(expected);
	});

	it('does not mutate the set it was given', () => {
		const original = [ref('openai', 'gpt-4o')];
		withDisabledRef(original, ref('anthropic', 'claude-3-haiku'));

		expect(original).toEqual([ref('openai', 'gpt-4o')]);
	});
});

describe('withoutDisabledRef', () => {
	it.each([
		{
			label: 'the only entry',
			refs: [ref('openai', 'gpt-4o')],
			ref: ref('openai', 'gpt-4o'),
			expected: []
		},
		{
			label: 'one entry and keeps the other provider',
			refs: [ref('openai', 'gpt-4o'), ref('anthropic', 'claude-3-5-sonnet')],
			ref: ref('openai', 'gpt-4o'),
			expected: [ref('anthropic', 'claude-3-5-sonnet')]
		},
		{
			label: 'nothing, because the ref is not held',
			refs: [ref('openai', 'gpt-4o')],
			ref: ref('anthropic', 'claude-3-5-sonnet'),
			expected: [ref('openai', 'gpt-4o')]
		},
		{
			label: 'nothing, from an empty set',
			refs: [],
			ref: ref('openai', 'gpt-4o'),
			expected: []
		},
		{
			label: 'every copy of a duplicated ref',
			refs: [ref('openai', 'gpt-4o'), ref('openai', 'gpt-4o'), ref('openai', 'gpt-4o-mini')],
			ref: ref('openai', 'gpt-4o'),
			expected: [ref('openai', 'gpt-4o-mini')]
		}
	])('removes $label', ({ refs, ref: entry, expected }) => {
		expect(withoutDisabledRef(refs, entry)).toEqual(expected);
	});

	it('does not mutate the set it was given', () => {
		const original = [ref('openai', 'gpt-4o'), ref('anthropic', 'claude-3-5-sonnet')];
		withoutDisabledRef(original, ref('openai', 'gpt-4o'));

		expect(original).toHaveLength(2);
	});
});

describe('providerDisabledRefs', () => {
	it.each([
		{
			label: 'one provider out of two',
			refs: [ref('openai', 'gpt-4o'), ref('anthropic', 'claude-3-5-sonnet')],
			provider: 'openai',
			expected: [ref('openai', 'gpt-4o')]
		},
		{
			label: 'a provider with nothing disabled',
			refs: [ref('openai', 'gpt-4o')],
			provider: 'anthropic',
			expected: []
		},
		{
			label: 'nothing from an empty set',
			refs: [],
			provider: 'openai',
			expected: []
		},
		{
			label: 'only the exact provider, not a prefix of it',
			refs: [ref('openai', 'gpt-4o'), ref('openai-free', 'gpt-4o')],
			provider: 'openai',
			expected: [ref('openai', 'gpt-4o')]
		},
		{
			label: 'every one of a provider, in order',
			refs: [
				ref('openai', 'gpt-4o'),
				ref('anthropic', 'claude-3-5-sonnet'),
				ref('openai', 'gpt-4o-mini')
			],
			provider: 'openai',
			expected: [ref('openai', 'gpt-4o'), ref('openai', 'gpt-4o-mini')]
		}
	])('narrows $label', ({ refs, provider, expected }) => {
		expect(providerDisabledRefs(refs, provider)).toEqual(expected);
	});
});

describe('schemaDisabledSet', () => {
	it('parses the answer the API sends, empty set included', () => {
		expect(schemaDisabledSet.parse({ data: [] })).toEqual({ data: [] });
		expect(schemaDisabledSet.parse({ data: [ref('openai', 'gpt-4o')] })).toEqual({
			data: [ref('openai', 'gpt-4o')]
		});
	});

	it.each([
		{ label: 'a missing data field', payload: {} },
		{ label: 'a ref missing its model', payload: { data: [{ provider_id: 'openai' }] } },
		{
			label: 'a ref with a blank provider',
			payload: { data: [{ provider_id: '', model_id: 'x' }] }
		},
		{
			label: 'a ref with a blank model',
			payload: { data: [{ provider_id: 'openai', model_id: '' }] }
		},
		{ label: 'a ref that is not an object', payload: { data: ['openai/gpt-4o'] } }
	])('refuses $label', ({ payload }) => {
		expect(schemaDisabledSet.safeParse(payload).success).toBe(false);
	});

	it('tolerates an additive field, because it parses a response', () => {
		const parsed = schemaDisabledSet.parse({
			data: [{ provider_id: 'openai', model_id: 'gpt-4o', disabled_at: '2026-09-20T03:00:00Z' }]
		});

		expect(parsed.data[0]).toMatchObject({ provider_id: 'openai', model_id: 'gpt-4o' });
	});
});

describe('schemaReplaceDisabledBody', () => {
	it('accepts the whole set, empty included', () => {
		expect(schemaReplaceDisabledBody.parse({ models: [] })).toEqual({ models: [] });
	});

	it.each([
		{ label: 'a missing models field', payload: {} },
		{ label: 'an unknown field', payload: { models: [], extra: true } },
		{ label: 'a model without a provider', payload: { models: [{ model_id: 'gpt-4o' }] } }
	])('refuses $label, because the panel builds this body', ({ payload }) => {
		expect(schemaReplaceDisabledBody.safeParse(payload).success).toBe(false);
	});
});
