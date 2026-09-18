// Model catalog schemas, mirroring docs/SPEC-API/001-SPEC-API.md §7.6 and the model catalog in
// docs/SPEC-UI/001-SPEC-UI.md §6.3.
//
// This is the source the provider detail screen's catalog reads, and the reason is the filters: §6.3 asks
// for a searchable list with `vision` and `tools` filters, and `GET /models/catalog` is the only route
// that accepts all three parameters (`provider_id`, `capability`, `q`). The provider-scoped route
// (`GET /providers/{id}/models`) accepts none of them, and its one extra field, `suggested`, is constant
// true in the embedded registry, so it cannot answer the "suggested" toggle §6.3 also asks for. That
// toggle is therefore absent rather than shipped as a control that filters nothing (R-26), and the gap is
// recorded here instead of worked around.
//
// `source` is carried because the catalog merges two origins: a `registry` row is the port's opinion
// about a model, a `custom` row is the operator's explicit statement about it. §6.3 puts custom models in
// U2, so every row is `registry` today, but the column renders from the value rather than assuming it.

import { z } from 'zod';
import { stringList } from './primitives';

// One merged catalog row. `kind` is optional because the DTO marks it `omitempty` and a custom model is
// built with an empty kind, so the field is genuinely absent rather than merely unknown. It is kept
// because a non-chat model is not routable through the chat data plane (§7.15), which is a distinction an
// operator choosing a model string needs to see.
export const schemaCatalogModel = z.object({
	id: z.string().min(1),
	provider_id: z.string().min(1),
	model_id: z.string().min(1),
	display_name: z.string(),
	kind: z.string().optional(),
	capabilities: stringList,
	source: z.string().min(1)
});

export type CatalogModel = z.infer<typeof schemaCatalogModel>;

// The catalog body. Not paginated: every row comes from an immutable in-memory index or from a table the
// operator curated by hand, so there is no bound to report (§7.6).
export const schemaModelCatalog = z.object({
	data: z.array(schemaCatalogModel)
});

export type ModelCatalog = z.infer<typeof schemaModelCatalog>;

// The three parameters the handler reads. `capability` is not narrowed to the two filters the panel
// offers: the API matches any capability string a model declares, and the panel's list is a convenience,
// not the API's vocabulary.
export const schemaCatalogQuery = z.strictObject({
	provider_id: z.string().optional(),
	capability: z.string().optional(),
	q: z.string().optional()
});

export type CatalogQuery = z.infer<typeof schemaCatalogQuery>;

// The two capability filters §6.3 names. Offered as the panel's filter vocabulary, not as the API's: the
// API matches any capability string a model declares, so a provider using a spelling outside this list is
// still reachable through `q` and still renders its own capabilities verbatim.
export const CATALOG_CAPABILITY_FILTERS = ['vision', 'tools'] as const;

// The filter state as the API's query. Pure, so the mapping is testable without a request: a blank or
// whitespace-only filter is dropped rather than sent as an empty parameter, which is what keeps the panel
// from asking the server a question it did not mean to ask.
//
// The panel does not re-filter the result. The API matches a capability case-insensitively and exactly,
// which is the behaviour the filter names promise, and a second pass here could only disagree with it.
export function catalogQueryParams(filters: {
	providerId?: string;
	capability?: string;
	query?: string;
}): CatalogQuery {
	const params: CatalogQuery = {};

	const providerId = filters.providerId?.trim() ?? '';
	if (providerId !== '') params.provider_id = providerId;

	const capability = filters.capability?.trim() ?? '';
	if (capability !== '') params.capability = capability;

	const query = filters.query?.trim() ?? '';
	if (query !== '') params.q = query;

	return params;
}

// The label a row leads with. Falls back to the model id because a row with no display name is a real
// shape: a custom model created without one still has to render something an operator can read.
export function catalogModelLabel(model: CatalogModel): string {
	const name = model.display_name.trim();
	return name === '' ? model.model_id : name;
}

// The origin of a row. An unknown source renders verbatim, so an origin the API adds later shows its own
// name rather than a blank cell.
export const CATALOG_SOURCE_LABELS: Record<string, string> = {
	registry: 'Registry',
	custom: 'Custom'
};

export function catalogSourceLabel(source: string): string {
	return CATALOG_SOURCE_LABELS[source] ?? source;
}
