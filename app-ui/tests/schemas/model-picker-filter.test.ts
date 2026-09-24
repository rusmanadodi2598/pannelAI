// The picker dialog's search (docs/SPEC-UI/001-SPEC-UI.md §6.4).
//
// Split from `model-picker.test.ts` by concern: that file holds the offer rules (who may appear), this one
// holds what the search does to the sections it is given.

import { describe, expect, it } from 'vitest';
import type { CatalogModel } from '$lib/schemas/model';
import { filterPickerSections, pickerOptions, pickerSections } from '$lib/schemas/model-picker';
import type { Provider } from '$lib/schemas/provider';

function provider(overrides: Partial<Provider> = {}): Provider {
	return {
		id: 'openai',
		name: 'OpenAI',
		category: 'apikey',
		auth_type: 'bearer',
		auth_modes: ['api_key'],
		has_oauth: false,
		no_auth: false,
		routability: 'native',
		endpoint_count: 1,
		status_summary: { total: 1, active: 1, disabled: 0, error: 0, rate_limited: 0 },
		...overrides
	};
}

function model(overrides: Partial<CatalogModel> = {}): CatalogModel {
	return {
		id: 'openai/gpt-4o',
		provider_id: 'openai',
		model_id: 'gpt-4o',
		display_name: 'GPT-4o',
		capabilities: [],
		source: 'registry',
		...overrides
	};
}

describe('filterPickerSections', () => {
	const sections = pickerSections({
		catalog: [
			model({
				id: 'openai/gpt-4o',
				provider_id: 'openai',
				model_id: 'gpt-4o',
				display_name: 'GPT-4o'
			}),
			model({
				id: 'openai/o3-mini',
				provider_id: 'openai',
				model_id: 'o3-mini',
				display_name: 'o3 mini'
			}),
			model({
				id: 'claude/claude-sonnet-4-5',
				provider_id: 'claude',
				model_id: 'claude-sonnet-4-5',
				display_name: 'Claude Sonnet 4.5'
			})
		],
		providers: [
			provider({ id: 'openai', name: 'OpenAI' }),
			provider({ id: 'claude', name: 'Claude' })
		],
		combos: ['daily']
	});

	it('returns everything for a blank query', () => {
		expect(filterPickerSections(sections, '   ')).toEqual(sections);
	});

	it('matches a model label and drops the sections that no longer match', () => {
		const filtered = filterPickerSections(sections, 'o3');

		expect(filtered.map((section) => section.key)).toEqual(['openai']);
		expect(filtered[0].options.map((option) => option.value)).toEqual(['openai/o3-mini']);
	});

	it('matches a section name and keeps all of its options', () => {
		const filtered = filterPickerSections(sections, 'claude');

		expect(filtered.map((section) => section.key)).toEqual(['claude']);
		expect(filtered[0].options).toHaveLength(1);
	});

	it('matches the stored ref, not only the label', () => {
		const filtered = filterPickerSections(sections, 'gpt-4o');

		expect(pickerOptions(filtered).map((option) => option.value)).toEqual(['openai/gpt-4o']);
	});
});
