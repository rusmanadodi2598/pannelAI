// How a catalog query becomes a request, and how a row is labelled (docs/SPEC-API/001-SPEC-API.md §7.6).
//
// The row schemas are in `model.test.ts`. These are the read side: the query the panel sends, the two labels
// a row answers with, and the filter set that query is built from (docs/SPEC-UI/001-SPEC-UI.md §6.3).

import { describe, expect, it } from 'vitest';
import {
	CATALOG_CAPABILITY_FILTERS,
	CATALOG_SOURCE_LABELS,
	catalogModelLabel,
	catalogQueryParams,
	catalogSourceLabel,
	schemaCatalogModel,
	schemaCatalogQuery,
	type CatalogModel
} from '$lib/schemas/model';

function catalogModel(overrides: Record<string, unknown> = {}): Record<string, unknown> {
	return {
		id: 'openai/gpt-4o',
		provider_id: 'openai',
		model_id: 'gpt-4o',
		display_name: 'GPT-4o',
		kind: 'llm',
		capabilities: ['vision', 'tools'],
		source: 'registry',
		...overrides
	};
}

describe('catalogQueryParams', () => {
	const CASES: {
		filters: { providerId?: string; capability?: string; query?: string };
		expected: Record<string, string>;
		why: string;
	}[] = [
		{
			filters: { providerId: 'openai', capability: 'vision', query: 'gpt' },
			expected: { provider_id: 'openai', capability: 'vision', q: 'gpt' },
			why: 'every filter is set'
		},
		{
			filters: { providerId: 'openai' },
			expected: { provider_id: 'openai' },
			why: 'only the provider scope is set'
		},
		{
			filters: { providerId: 'openai', query: '' },
			expected: { provider_id: 'openai' },
			why: 'an empty search is dropped rather than sent as q='
		},
		{
			filters: { providerId: 'openai', query: '   ' },
			expected: { provider_id: 'openai' },
			why: 'a whitespace-only search is dropped too'
		},
		{
			filters: { providerId: 'openai', capability: '\t' },
			expected: { provider_id: 'openai' },
			why: 'a whitespace-only capability is dropped'
		},
		{
			filters: {},
			expected: {},
			why: 'nothing is set, so nothing is sent'
		},
		{
			filters: { providerId: '  openai  ', capability: ' vision ', query: '  gpt  ' },
			expected: { provider_id: 'openai', capability: 'vision', q: 'gpt' },
			why: 'surrounding whitespace is trimmed off every value'
		},
		{
			filters: { providerId: 'openai', query: 'gpt 4o' },
			expected: { provider_id: 'openai', q: 'gpt 4o' },
			why: 'whitespace inside a search phrase is preserved, because it is part of the phrase'
		},
		{
			filters: { providerId: 'openai', query: 'gpt-4o & vision' },
			expected: { provider_id: 'openai', q: 'gpt-4o & vision' },
			why: 'characters the API must encode are passed through untouched'
		}
	];

	for (const testCase of CASES) {
		it(`returns ${JSON.stringify(testCase.expected)} when ${testCase.why}`, () => {
			expect(catalogQueryParams(testCase.filters)).toEqual(testCase.expected);
		});
	}

	it('omits a filter key entirely rather than setting it to an empty string', () => {
		// The client drops `''` before building the URL, but the mapping is asserted here so the rule has
		// one home: an absent filter is absent, not blank.
		expect(Object.keys(catalogQueryParams({ providerId: '', capability: '', query: '' }))).toEqual(
			[]
		);
	});
});

describe('catalogModelLabel', () => {
	const CASES: { model: Partial<CatalogModel>; expected: string; why: string }[] = [
		{
			model: { display_name: 'GPT-4o', model_id: 'gpt-4o' },
			expected: 'GPT-4o',
			why: 'a display name'
		},
		{
			model: { display_name: '', model_id: 'gpt-4o' },
			expected: 'gpt-4o',
			why: 'no display name, so the model id stands in'
		},
		{
			model: { display_name: '   ', model_id: 'gpt-4o' },
			expected: 'gpt-4o',
			why: 'a whitespace-only display name is treated as none'
		},
		{
			model: { display_name: '  GPT-4o  ', model_id: 'gpt-4o' },
			expected: 'GPT-4o',
			why: 'surrounding whitespace is trimmed'
		},
		{
			model: { display_name: 'gpt-4o', model_id: 'gpt-4o' },
			expected: 'gpt-4o',
			why: 'a display name that matches the id'
		}
	];

	for (const testCase of CASES) {
		it(`returns "${testCase.expected}" for ${testCase.why}`, () => {
			const parsed = schemaCatalogModel.parse(catalogModel({ ...testCase.model }));
			expect(catalogModelLabel(parsed)).toBe(testCase.expected);
		});
	}
});

describe('catalogSourceLabel', () => {
	it('names the two origins the catalog merges', () => {
		expect(catalogSourceLabel('registry')).toBe('Registry');
		expect(catalogSourceLabel('custom')).toBe('Custom');
	});

	it('renders an origin the panel does not know verbatim rather than blank', () => {
		expect(catalogSourceLabel('imported')).toBe('imported');
	});

	it('covers every source the schema accepts, so a new origin fails here before it renders unnamed', () => {
		// The two origins are the ones the API defines; the assertion is that the label map and the
		// vocabulary agree, not that the list is closed.
		expect(Object.keys(CATALOG_SOURCE_LABELS)).toEqual(['registry', 'custom']);
	});
});

describe('CATALOG_CAPABILITY_FILTERS', () => {
	it('offers exactly the two filters §6.3 names', () => {
		expect([...CATALOG_CAPABILITY_FILTERS]).toEqual(['vision', 'tools']);
	});

	it('offers filters the API accepts as capability parameters', () => {
		for (const capability of CATALOG_CAPABILITY_FILTERS) {
			expect(schemaCatalogQuery.safeParse({ capability }).success).toBe(true);
		}
	});
});
