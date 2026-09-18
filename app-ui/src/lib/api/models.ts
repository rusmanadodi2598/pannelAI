// Model catalog calls, mirroring docs/SPEC-API/001-SPEC-API.md §7.6.
//
// Only the read route is declared. §6.3 places the catalog's writes (custom models, the alias set, the
// disabled set) in U2, and a call the panel cannot reach is a route it should not declare.

import { schemaModelCatalog, type CatalogQuery, type ModelCatalog } from '$lib/schemas/model';
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
