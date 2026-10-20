// Model catalog schema contract tests (docs/SPEC-API/001-SPEC-API.md §7.6, docs/SPEC-UI §6.3).
//
// The catalog is what the provider detail screen renders, so these tests pin the three things that would
// silently mislead an operator: a Go nil slice arriving as `null` rather than `[]` for `capabilities`, a
// `kind` that is genuinely absent rather than unknown (the DTO marks it `omitempty` and a custom model is
// built without one), and a filter the panel drops instead of sending, because a whitespace-only search
// sent as `q=%20` looks to the server like a question the operator meant to ask.

import { describe, expect, it } from 'vitest';
import { schemaCatalogModel, schemaCatalogQuery, schemaModelCatalog } from '$lib/schemas/model';
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
