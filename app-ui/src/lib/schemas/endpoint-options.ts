// The provider option list the upstream endpoints filter offers (docs/SPEC-UI/001-SPEC-UI.md §6.2, tab 2).
//
// The options are derived from an unfiltered read of the same list route rather than from the filtered
// result the table shows. Deriving them from the result would strand the operator on the provider they
// picked: every other provider would leave the select until the filter was cleared, and switching provider
// would mean clearing first. §6.3's pagination discipline still holds, because the read is the endpoint
// list with the server's own paging, not the provider registry pulled in to filter locally.
//
// The name falls back to the provider id, which is the answer the table already gives a row whose provider
// the registry does not name. An id can be acted on; a blank option cannot.

import type { Endpoint } from './endpoint';

/** The API caps per_page at 100 (SPEC-API §4), which is the cap this read asks for. */
export const OPTION_PAGE_SIZE = 100;

export type ProviderOption = { id: string; name: string };

/** The distinct providers of a set of endpoint rows, deduplicated, named, and sorted for a select. */
export function toProviderOptions(rows: Endpoint[]): ProviderOption[] {
	const names = new Map(rows.map((row) => [row.provider_id, row.provider_name ?? row.provider_id]));
	return [...names.entries()]
		.map(([id, name]) => ({ id, name }))
		.sort((left, right) => left.name.localeCompare(right.name));
}

/**
 * The option list with the provider the URL names always present.
 *
 * A select whose value has no matching option renders blank, which would hide the filter in force when the
 * address names a provider no read returned, so the id is offered as its own name.
 */
export function withSelectedProvider(
	options: ProviderOption[],
	providerId: string
): ProviderOption[] {
	if (providerId === '' || options.some((option) => option.id === providerId)) return options;
	return [...options, { id: providerId, name: providerId }];
}
