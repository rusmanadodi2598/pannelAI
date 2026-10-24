// The bulk mutation wire shapes: one selector that names a set, and the answer that says what it did
// (docs/SPEC-API/001-SPEC-API.md §7.5, §8.1, SPEC-UI §6.2).
//
// The selector is the whole safety argument of these routes, so it is declared once and shared by all
// eight: exactly one of `ids`, `tag`, or `all`. The server answers with the count IT resolved rather
// than echoing the caller's, because a page of ten rows out of forty-seven is the number a Select-All
// has to be honest about.

import { z } from 'zod';

/** One bulk row's verdict. A refused row carries its own code, so a client can map it without reading prose. */
export const schemaBulkMutationRow = z.object({
	index: z.number().int(),
	id: z.string().optional(),
	error: z.string().optional(),
	code: z.string().optional()
});

export type BulkMutationRow = z.infer<typeof schemaBulkMutationRow>;

/** The narrowing axes every bulk selector carries. */
export const schemaBulkSelector = z.object({
	ids: z.array(z.string()).max(200).optional(),
	tag: z.string().max(48).optional(),
	auth_type: z.enum(['api_key', 'oauth', 'no_auth', 'service_account']).optional(),
	provider_id: z.string().max(120).optional(),
	all: z.literal(true).optional()
});

export type BulkSelector = z.infer<typeof schemaBulkSelector>;

/** The answer to a bulk mutation: what the server matched, what it changed, and each row's verdict. */
export const schemaBulkMutationResult = z.object({
	matched: z.number().int(),
	changed: z.number().int(),
	results: z.array(schemaBulkMutationRow)
});

export type BulkMutationResult = z.infer<typeof schemaBulkMutationResult>;

/** What a proxy erase took down with it: the endpoints left without an egress path and the providers losing a pin. */
export const schemaProxyPins = z.object({
	endpoints: z.number().int(),
	providers: z.array(z.string())
});

export type ProxyPins = z.infer<typeof schemaProxyPins>;

/** The answer to a proxy bulk route, which carries the pin report beside the rows. */
export const schemaBulkProxyResult = schemaBulkMutationResult.extend({
	pins: schemaProxyPins
});

export type BulkProxyResult = z.infer<typeof schemaBulkProxyResult>;

/** A request body that must name exactly one set: the three axes are present but never together. */
export function bulkSelectorBody(selector: BulkSelector): BulkSelector {
	if (selector.ids?.length) {
		return { ids: selector.ids };
	}
	if (selector.tag) {
		return {
			tag: selector.tag,
			...(selector.provider_id ? { provider_id: selector.provider_id } : {}),
			...(selector.auth_type ? { auth_type: selector.auth_type } : {})
		};
	}
	return {
		all: true,
		...(selector.provider_id ? { provider_id: selector.provider_id } : {}),
		...(selector.auth_type ? { auth_type: selector.auth_type } : {})
	};
}
