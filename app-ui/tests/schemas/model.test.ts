// Model catalog schema contract tests (docs/SPEC-API/001-SPEC-API.md §7.6, docs/SPEC-UI §6.3).
//
// The catalog is what the provider detail screen renders, so these tests pin the three things that would
// silently mislead an operator: a Go nil slice arriving as `null` rather than `[]` for `capabilities`, a
// `kind` that is genuinely absent rather than unknown (the DTO marks it `omitempty` and a custom model is
// built without one), and a filter the panel drops instead of sending, because a whitespace-only search
// sent as `q=%20` looks to the server like a question the operator meant to ask.

import { describe, expect, it } from 'vitest';
import {
	CATALOG_CAPABILITY_FILTERS,
	CATALOG_SOURCE_LABELS,
	catalogModelLabel,
	catalogQueryParams,
	catalogSourceLabel,
	schemaCatalogModel,
	schemaCatalogQuery,
	schemaModelCatalog,
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

describe('schemaCatalogModel', () => {
	it('accepts a full registry row', () => {
		expect(schemaCatalogModel.safeParse(catalogModel()).success).toBe(true);
	});

	it('accepts capabilities as null, which is how Go marshals a nil slice, and normalizes to empty', () => {
		const result = schemaCatalogModel.safeParse(catalogModel({ capabilities: null }));

		expect(result.success).toBe(true);
		expect(result.success && result.data.capabilities).toEqual([]);
	});

	it('accepts an omitted capabilities field, normalizing to empty the same way', () => {
		const payload = catalogModel();
		delete payload.capabilities;

		const result = schemaCatalogModel.safeParse(payload);

		expect(result.success).toBe(true);
		expect(result.success && result.data.capabilities).toEqual([]);
	});

	it('accepts an omitted kind, which is the shape a custom model arrives in', () => {
		// The DTO marks `kind` omitempty and the service builds a custom row with an empty kind, so the
		// field is genuinely absent. Requiring it would fail the whole catalog on the first custom model.
		const payload = catalogModel({ source: 'custom' });
		delete payload.kind;

		const result = schemaCatalogModel.safeParse(payload);

		expect(result.success).toBe(true);
		expect(result.success && result.data.kind).toBeUndefined();
	});

	it('accepts a model with no declared capabilities', () => {
		const result = schemaCatalogModel.safeParse(catalogModel({ capabilities: [] }));

		expect(result.success).toBe(true);
		expect(result.success && result.data.capabilities).toEqual([]);
	});

	it('accepts a custom row, whose source differs from the registry', () => {
		expect(schemaCatalogModel.safeParse(catalogModel({ source: 'custom' })).success).toBe(true);
	});

	it('accepts an empty display name, because the label falls back to the model id', () => {
		expect(schemaCatalogModel.safeParse(catalogModel({ display_name: '' })).success).toBe(true);
	});

	const INVALID: { payload: Record<string, unknown>; why: string }[] = [
		{ payload: catalogModel({ id: '' }), why: 'an empty catalog id' },
		{ payload: catalogModel({ provider_id: '' }), why: 'an empty provider id' },
		{ payload: catalogModel({ model_id: '' }), why: 'an empty model id' },
		{ payload: catalogModel({ source: '' }), why: 'an empty source' },
		{
			payload: catalogModel({ capabilities: ['vision', 7] }),
			why: 'a capability that is not a string'
		},
		{ payload: catalogModel({ capabilities: 'vision' }), why: 'a capability list sent as a string' }
	];

	for (const testCase of INVALID) {
		it(`rejects ${testCase.why}`, () => {
			expect(schemaCatalogModel.safeParse(testCase.payload).success).toBe(false);
		});
	}
});

describe('schemaModelCatalog', () => {
	it('accepts a page of rows', () => {
		expect(schemaModelCatalog.safeParse({ data: [catalogModel()] }).success).toBe(true);
	});

	it('accepts an empty catalog', () => {
		const result = schemaModelCatalog.safeParse({ data: [] });

		expect(result.success).toBe(true);
		expect(result.success && result.data.data).toEqual([]);
	});

	it('requires the data key, because the catalog is never paginated and carries no meta to fall back on', () => {
		expect(schemaModelCatalog.safeParse({}).success).toBe(false);
	});
});

describe('schemaCatalogQuery', () => {
	const ACCEPTED: { query: Record<string, unknown>; why: string }[] = [
		{ query: {}, why: 'no filter at all' },
		{ query: { provider_id: 'openai' }, why: 'the provider scope' },
		{ query: { capability: 'vision' }, why: 'the vision filter' },
		{ query: { capability: 'tools' }, why: 'the tools filter' },
		{ query: { q: 'gpt' }, why: 'a search term' },
		{
			query: { provider_id: 'openai', capability: 'vision', q: 'gpt' },
			why: 'all three together'
		},
		{
			query: { capability: 'reasoning' },
			why: 'a capability outside the two the panel offers, which the API still matches'
		}
	];

	for (const testCase of ACCEPTED) {
		it(`accepts ${testCase.why}`, () => {
			expect(schemaCatalogQuery.safeParse(testCase.query).success).toBe(true);
		});
	}

	it('rejects a parameter the handler does not read, so the panel cannot send a filter that is ignored', () => {
		expect(schemaCatalogQuery.safeParse({ provider_id: 'openai', limit: 10 }).success).toBe(false);
	});
});

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
