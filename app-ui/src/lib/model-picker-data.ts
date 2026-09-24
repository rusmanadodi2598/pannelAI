// The reads behind the reference picker (docs/SPEC-UI/001-SPEC-UI.md §6.4).
//
// Two screens ask the same question ("which refs can this operator pick right now?"), so the reads live
// here rather than in each of them: the catalog supplies the refs, the provider list supplies which of
// them are configured, and `pickerSections` joins the two. The sources are returned rather than the
// sections, because the combo tab's section list also carries its combos, which change with every list
// read: a caller derives the sections so the two cannot drift apart.
//
// The provider list is read page by page rather than trusted to one page: the route caps `per_page` at
// 100 (SPEC-API §4) and the registry is 86 rows today, so a second page is one request away from being
// real. A read that fails anywhere reports `failed` instead of answering a shorter list, because a
// silently truncated active set would hide a provider the operator did configure.

import { listModelCatalog } from '$lib/api/models';
import { listProviders } from '$lib/api/providers';
import type { CatalogModel, CatalogQuery } from '$lib/schemas/model';
import type { Provider } from '$lib/schemas/provider';

export type PickerSources = {
	catalog: CatalogModel[];
	providers: Provider[];
	/** True when a read failed, so the dialog can say so instead of presenting an empty catalog. */
	failed: boolean;
};

const PROVIDER_PAGE_SIZE = 100;

// Every provider the list route will serve, or null when a page could not be read.
async function readAllProviders(): Promise<Provider[] | null> {
	const first = await listProviders({ page: 1, per_page: PROVIDER_PAGE_SIZE });
	if (!first.ok) return null;

	const rows: Provider[] = [...first.data.data];
	const total = first.data.meta.total;
	let page = 2;

	while (rows.length < total) {
		const next = await listProviders({ page, per_page: PROVIDER_PAGE_SIZE });
		if (!next.ok) return null;
		if (next.data.data.length === 0) break;
		rows.push(...next.data.data);
		page += 1;
	}

	return rows;
}

// The picker's two sources for one catalog query. A failure is reported rather than thrown: the caller
// decides how to say it, and the editor stays usable because every ref is still typeable by hand.
export async function loadPickerSources(query: CatalogQuery = {}): Promise<PickerSources> {
	const [providers, catalog] = await Promise.all([readAllProviders(), listModelCatalog(query)]);

	if (providers === null || !catalog.ok) {
		return {
			catalog: catalog.ok ? catalog.data.data : [],
			providers: providers ?? [],
			failed: true
		};
	}

	return { catalog: catalog.data.data, providers, failed: false };
}
