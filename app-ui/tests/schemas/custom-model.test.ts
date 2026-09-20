// Tests for the custom model contracts (docs/SPEC-UI/001-SPEC-UI.md §6.3).
//
// The form's rules are the API's own bounds, so each case states the boundary the API documents: an empty
// required field, a model id past 200 characters, a display name past 120, and the capability list's two
// bounds. The parser is tested separately because a trailing comma or a stray space is what an operator
// actually types.

import { describe, expect, it } from 'vitest';
import {
	CAPABILITY_MAX_ENTRIES,
	CAPABILITY_MAX_LENGTH,
	capabilitiesText,
	customModelBody,
	customModelDraftEmpty,
	customModelLabel,
	parseCapabilities,
	providerCustomModels,
	schemaCustomModel,
	schemaCustomModelForm,
	type CustomModel
} from '$lib/schemas/custom-model';

const row = (overrides: Partial<CustomModel> = {}): CustomModel => ({
	id: 'mdl_01',
	provider_id: 'openai',
	model_id: 'gpt-4o-mini',
	display_name: 'GPT-4o mini',
	capabilities: ['tools'],
	created_at: '2026-09-20T03:00:00Z',
	...overrides
});

describe('parseCapabilities', () => {
	it.each([
		{ label: 'an empty field', text: '', expected: [] },
		{ label: 'whitespace only', text: '   ', expected: [] },
		{ label: 'one value', text: 'vision', expected: ['vision'] },
		{ label: 'a trailing comma', text: 'vision,', expected: ['vision'] },
		{
			label: 'surrounding and internal spaces',
			text: ' vision , tools ',
			expected: ['vision', 'tools']
		},
		{ label: 'a doubled comma', text: 'vision,,tools', expected: ['vision', 'tools'] },
		{
			label: 'duplicates, keeping the first place',
			text: 'tools, vision, tools',
			expected: ['tools', 'vision']
		},
		{ label: 'case, which is preserved', text: 'Vision,TOOLS', expected: ['Vision', 'TOOLS'] },
		{
			label: 'a value with an inner space, which is kept',
			text: 'json mode',
			expected: ['json mode']
		},
		{ label: 'a long list', text: 'a,b,c,d,e,f,g', expected: ['a', 'b', 'c', 'd', 'e', 'f', 'g'] }
	])('parses $label', ({ text, expected }) => {
		expect(parseCapabilities(text)).toEqual(expected);
	});
});

describe('capabilitiesText', () => {
	it.each([
		{ label: 'an empty list', list: [], expected: '' },
		{ label: 'one value', list: ['vision'], expected: 'vision' },
		{ label: 'two values', list: ['vision', 'tools'], expected: 'vision, tools' }
	])('renders $label', ({ list, expected }) => {
		expect(capabilitiesText(list)).toBe(expected);
	});

	it('round trips through the parser', () => {
		const list = ['vision', 'tools', 'json mode'];

		expect(parseCapabilities(capabilitiesText(list))).toEqual(list);
	});
});

describe('schemaCustomModelForm', () => {
	const valid = { model_id: 'gpt-4o-mini', display_name: 'GPT-4o mini', capabilities: 'vision' };

	it('accepts a filled form and trims what it trims', () => {
		expect(schemaCustomModelForm.parse(valid)).toEqual(valid);
		expect(schemaCustomModelForm.parse({ ...valid, model_id: '  gpt-4o-mini  ' }).model_id).toBe(
			'gpt-4o-mini'
		);
		expect(schemaCustomModelForm.parse({ ...valid, capabilities: '' }).capabilities).toBe('');
	});

	it.each([
		{ label: 'an empty model id', form: { ...valid, model_id: '' } },
		{ label: 'a model id that is whitespace', form: { ...valid, model_id: '   ' } },
		{ label: 'a model id with an inner space', form: { ...valid, model_id: 'gpt 4o' } },
		{ label: 'a model id past the API bound', form: { ...valid, model_id: 'a'.repeat(201) } },
		{ label: 'an empty display name', form: { ...valid, display_name: '' } },
		{
			label: 'a display name past the API bound',
			form: { ...valid, display_name: 'a'.repeat(121) }
		},
		{ label: 'a display name with angle brackets', form: { ...valid, display_name: '<b>x</b>' } },
		{ label: 'an unknown field', form: { ...valid, provider_id: 'anthropic' } }
	])('refuses $label', ({ form }) => {
		expect(schemaCustomModelForm.safeParse(form).success).toBe(false);
	});

	it('accepts a model id at the API bound', () => {
		expect(schemaCustomModelForm.safeParse({ ...valid, model_id: 'a'.repeat(200) }).success).toBe(
			true
		);
	});

	it.each([
		{
			label: 'the entry-count bound',
			capabilities: Array.from({ length: CAPABILITY_MAX_ENTRIES }, (_, i) => `c${i}`).join(','),
			ok: true
		},
		{
			label: 'one entry past it',
			capabilities: Array.from({ length: CAPABILITY_MAX_ENTRIES + 1 }, (_, i) => `c${i}`).join(','),
			ok: false
		},
		{
			label: 'an entry at the length bound',
			capabilities: 'a'.repeat(CAPABILITY_MAX_LENGTH),
			ok: true
		},
		{
			label: 'an entry past it',
			capabilities: 'a'.repeat(CAPABILITY_MAX_LENGTH + 1),
			ok: false
		}
	])('checks $label', ({ capabilities, ok }) => {
		expect(schemaCustomModelForm.safeParse({ ...valid, capabilities }).success).toBe(ok);
	});
});

describe('customModelBody', () => {
	it('takes the provider from the screen and the rest from the form', () => {
		const body = customModelBody('anthropic', {
			model_id: 'claude-3-haiku',
			display_name: 'Claude 3 Haiku',
			capabilities: 'vision, tools, vision'
		});

		expect(body).toEqual({
			provider_id: 'anthropic',
			model_id: 'claude-3-haiku',
			display_name: 'Claude 3 Haiku',
			capabilities: ['vision', 'tools']
		});
	});

	it('sends an empty list when nothing was typed', () => {
		expect(customModelBody('openai', customModelDraftEmpty()).capabilities).toEqual([]);
	});
});

describe('providerCustomModels', () => {
	it.each([
		{
			label: 'one provider out of two',
			rows: [row(), row({ id: 'mdl_02', provider_id: 'anthropic' })],
			provider: 'openai',
			expected: 1
		},
		{ label: 'a provider with none', rows: [row()], provider: 'anthropic', expected: 0 },
		{ label: 'an empty list', rows: [], provider: 'openai', expected: 0 },
		{
			label: 'only the exact provider, not a prefix',
			rows: [row(), row({ id: 'mdl_03', provider_id: 'openai-free' })],
			provider: 'openai',
			expected: 1
		}
	])('narrows $label', ({ rows, provider, expected }) => {
		expect(providerCustomModels(rows, provider)).toHaveLength(expected);
	});
});

describe('customModelLabel', () => {
	it.each([
		{ label: 'the display name', row: row(), expected: 'GPT-4o mini' },
		{
			label: 'the model id when no name is stored',
			row: row({ display_name: '' }),
			expected: 'gpt-4o-mini'
		},
		{
			label: 'the model id when the name is only spaces',
			row: row({ display_name: '  ' }),
			expected: 'gpt-4o-mini'
		}
	])('uses $label', ({ row: entry, expected }) => {
		expect(customModelLabel(entry)).toBe(expected);
	});
});

describe('schemaCustomModel', () => {
	it('parses a row the API sends', () => {
		expect(schemaCustomModel.parse(row())).toMatchObject({ id: 'mdl_01', model_id: 'gpt-4o-mini' });
	});

	it('normalizes a null capability list, which Go sends for an empty slice', () => {
		expect(
			schemaCustomModel.parse(row({ capabilities: null as unknown as string[] })).capabilities
		).toEqual([]);
	});

	it.each([
		{ label: 'a row without an id', payload: { ...row(), id: '' } },
		{ label: 'a row without a created_at', payload: { ...row(), created_at: undefined } },
		{
			label: 'a row with an unparseable created_at',
			payload: { ...row(), created_at: 'yesterday' }
		}
	])('refuses $label', ({ payload }) => {
		expect(schemaCustomModel.safeParse(payload).success).toBe(false);
	});
});
