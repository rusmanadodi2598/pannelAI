// Model catalog calls, mirroring docs/SPEC-API/001-SPEC-API.md §7.6.
//
// §7.6 is four resources over one screen (docs/SPEC-UI/001-SPEC-UI.md §6.3): the merged catalog, the
// custom rows, the alias set, and the disabled set. The catalog read is the only one the panel needs a
// query for; the other three come back whole, because none of them is paginated.
//
// The two set writes are authoritative replaces, so every caller hands over the WHOLE set it last read
// rather than a delta. The answer to each write is the new set, which is what the panel renders: the
// server's own state, not the panel's reconstruction of it.

import {
	schemaCustomModel,
	schemaCustomModelList,
	type CreateCustomModelBody,
	type CustomModel,
	type CustomModelList
} from '$lib/schemas/custom-model';
import {
	aliasSetBody,
	schemaAliasSet,
	schemaReplaceAliasesBody,
	type AliasSet,
	type ModelAliasEntry
} from '$lib/schemas/model-alias';
import {
	schemaDisabledSet,
	schemaReplaceDisabledBody,
	type DisabledRef,
	type DisabledSet
} from '$lib/schemas/model-disabled';
import { schemaModelCatalog, type CatalogQuery, type ModelCatalog } from '$lib/schemas/model';
import { emptyResponse, type EmptyResponse } from '$lib/schemas/primitives';
import { apiRequest, type ApiResult } from './client';

// The merged catalog, narrowed by the three parameters the API reads. It is not paginated, so the result
// is a plain list rather than a page: the bound is the registry's size, not a customer's data.
export function listModelCatalog(query: CatalogQuery = {}): Promise<ApiResult<ModelCatalog>> {
	return apiRequest<void, ModelCatalog>({
		method: 'GET',
		path: '/models/catalog',
		schema: schemaModelCatalog,
		query
	});
}

// The disabled set, every provider's rows in one answer. The route reads no filter, so the panel narrows
// it for display and must keep the whole set for the write below.
export function listDisabledModels(): Promise<ApiResult<DisabledSet>> {
	return apiRequest<void, DisabledSet>({
		method: 'GET',
		path: '/models/disabled',
		schema: schemaDisabledSet
	});
}

// Replaces the disabled set. The body is the whole set, so a caller that hands over one provider's slice
// deletes every other provider's rows: `withDisabledRef` and `withoutDisabledRef` exist to make the merge
// over the whole set the only easy thing to write.
export function replaceDisabledModels(refs: DisabledRef[]): Promise<ApiResult<DisabledSet>> {
	return apiRequest<{ models: DisabledRef[] }, DisabledSet>({
		method: 'PUT',
		path: '/models/disabled',
		schema: schemaDisabledSet,
		body: { models: refs },
		bodySchema: schemaReplaceDisabledBody
	});
}

// Every custom row, across providers. Narrowed for display like the disabled set, and read for display
// only: a custom row is written one at a time, not as a set.
export function listCustomModels(): Promise<ApiResult<CustomModelList>> {
	return apiRequest<void, CustomModelList>({
		method: 'GET',
		path: '/models/custom',
		schema: schemaCustomModelList
	});
}

// Adds one custom model. The API validates the provider against the registry and answers CONFLICT when
// the pair is already declared, so the panel does not predict either answer.
export function createCustomModel(body: CreateCustomModelBody): Promise<ApiResult<CustomModel>> {
	return apiRequest<CreateCustomModelBody, CustomModel>({
		method: 'POST',
		path: '/models/custom',
		schema: schemaCustomModel,
		body
	});
}

// Removes one custom row by its `mdl_` id. The route answers 204, so there is no body to parse.
export function deleteCustomModel(id: string): Promise<ApiResult<EmptyResponse>> {
	return apiRequest<void, EmptyResponse>({
		method: 'DELETE',
		path: `/models/custom/${encodeURIComponent(id)}`,
		schema: emptyResponse
	});
}

// The whole alias set. Global rather than per provider: the route reads no filter, which is why the screen
// that renders it says so and why the merge below is over every alias rather than over a slice.
export function listModelAliases(): Promise<ApiResult<AliasSet>> {
	return apiRequest<void, AliasSet>({
		method: 'GET',
		path: '/models/aliases',
		schema: schemaAliasSet
	});
}

// Replaces the whole alias set. The body is built from every entry the caller last read, sorted the way the
// read route sorts, so the answer is the set the panel already shows in the order it shows it.
export function replaceModelAliases(entries: ModelAliasEntry[]): Promise<ApiResult<AliasSet>> {
	return apiRequest<{ aliases: ModelAliasEntry[] }, AliasSet>({
		method: 'PUT',
		path: '/models/aliases',
		schema: schemaAliasSet,
		body: aliasSetBody(entries),
		bodySchema: schemaReplaceAliasesBody
	});
}
