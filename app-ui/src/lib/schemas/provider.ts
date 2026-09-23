// Provider registry schemas, mirroring docs/SPEC-API/001-SPEC-API.md §7.4 and the table in
// docs/SPEC-UI/001-SPEC-UI.md §6.3.
//
// `category`, `routability`, and `kind` are plain strings rather than enums. The registry treats category
// as an open string, so a panel that enumerated it would fail to render a provider the API had just
// returned; the panel filters by the values it sees and shows the rest verbatim. §14 Q9 records the same
// decision for a gateway key's status.
//
// Two model routes are read from a provider's detail screen and they are not the same list. §7.6's
// catalog carries the filters the screen offers, so its shapes live in `model.ts`. §7.4's
// `/providers/{provider_id}/models` is the provider's own answer, which is the only list a custom node
// has: its models come from the base URL it points at rather than from the embedded registry. The shapes
// for that route are here, beside the provider they belong to.

import { z } from 'zod';
import { label, stringList } from './primitives';

// The pagination block SPEC-API §4 returns beside a paginated list.
const pageMeta = z.object({
	page: z.number().int(),
	per_page: z.number().int(),
	total: z.number().int()
});

// How the stored accounts for one provider are doing. A nested block rather than three sibling fields,
// because the list renders it as one thing.
export const schemaProviderStatusSummary = z.object({
	total: z.number().int().min(0),
	active: z.number().int().min(0),
	disabled: z.number().int().min(0),
	error: z.number().int().min(0),
	rate_limited: z.number().int().min(0)
});

export type ProviderStatusSummary = z.infer<typeof schemaProviderStatusSummary>;

export const schemaProvider = z.object({
	id: z.string().min(1),
	name: z.string().min(1),
	category: z.string().min(1),
	auth_type: z.string(),
	auth_modes: stringList,
	has_oauth: z.boolean(),
	no_auth: z.boolean(),
	routability: z.string(),
	endpoint_count: z.number().int().min(0),
	status_summary: schemaProviderStatusSummary
});

export type Provider = z.infer<typeof schemaProvider>;

export const schemaProviderList = z.object({
	data: z.array(schemaProvider),
	meta: pageMeta
});

export type ProviderList = z.infer<typeof schemaProviderList>;

// One non-chat service kind's endpoint and credential placement. `auth_header` is reported because the
// value `key` means a query parameter rather than a header.
export const schemaProviderMedia = z.object({
	kind: z.string().min(1),
	base_url: z.string(),
	auth_type: z.string(),
	auth_header: z.string(),
	format: z.string(),
	default_model: z.string(),
	model_count: z.number().int().min(0)
});

export type ProviderMedia = z.infer<typeof schemaProviderMedia>;

// The detail body embeds the list shape, so it repeats every list field plus the transport defaults. Those
// defaults are what decide reachability, which is why the screen shows them rather than the display name.
//
// `media` is declared because the API sends it and a shape the panel does not describe is a shape it
// cannot notice changing. It is not rendered on this screen: §6.3's header names five facts and media is
// not among them, and §6.8 owns the per-kind view.
export const schemaProviderDetail = schemaProvider.extend({
	base_url: z.string(),
	format: z.string(),
	url_suffix: z.string(),
	validate_url: z.string(),
	timeout_ms: z.number().int(),
	model_count: z.number().int().min(0),
	chat_model_count: z.number().int().min(0),
	media: z
		.array(schemaProviderMedia)
		.nullish()
		.transform((value) => value ?? []),
	deprecated: z.boolean(),
	deprecation_notice: z.string().optional(),
	website: z.string().optional()
});

export type ProviderDetail = z.infer<typeof schemaProviderDetail>;

// One model the provider's own `/models` answer carries (§7.4). A node's rows come from the upstream its
// base URL points at; a registry provider answers the rows the registry already declares.
//
// `name`, `kind`, `capabilities`, and `dimensions` are optional because the route's own row type omits
// them when it has nothing to say, and the panel reads only `id` and `name`. `source` and `warning` state
// where the list came from, which is not decoration: a node whose upstream could not be reached answers a
// fallback, and a client that could not tell that from a fresh answer would present a stale list as
// current. Both are optional because the gateway omits them when the list is the upstream's own
// (app-serv draft 017 F2 carries them), and the panel renders whichever it receives.
export const schemaProviderModel = z.object({
	id: z.string().min(1),
	name: z.string().optional(),
	kind: z.string().optional(),
	capabilities: stringList.optional(),
	dimensions: z.number().int().optional()
});

export type ProviderModel = z.infer<typeof schemaProviderModel>;

export const schemaProviderModelList = z.object({
	data: z.array(schemaProviderModel),
	source: z.string().optional(),
	warning: z.string().optional()
});

export type ProviderModelList = z.infer<typeof schemaProviderModelList>;

// The categories the reference registry measures. Offered as filter suggestions, not as a closed set: the
// list the API returns is authoritative, and a provider with a category outside this list still renders.
export const PROVIDER_CATEGORIES = ['apikey', 'oauth', 'free', 'media', 'local'] as const;

// The list query, matching exactly what the API reads: `category`, `routability`, and paging.
//
// §6.3 also asks for a search over name and ID. The API does not accept one: `ProviderListQuery` declares
// only category and routability, and the handler reads no other filter. §6.3's own pagination discipline
// forbids the alternative of pulling the registry into a client-side filter, and R-26 forbids shipping a
// search box that cannot search. So the control is absent until the API grows the parameter, which is
// recorded as a gap rather than worked around here.
export const schemaProviderQuery = z.strictObject({
	category: z.string().optional(),
	routability: z.enum(['native', 'connector']).optional()
});

export type ProviderQuery = z.infer<typeof schemaProviderQuery>;

// The one-line status a provider row shows. Written as a sentence rather than a set of numbers because the
// operator's question is whether anything needs attention, not what the exact counts are.
export function statusSummaryText(summary: ProviderStatusSummary): string {
	if (summary.total === 0) return 'No endpoint configured';
	if (summary.error > 0) return `${summary.error} failing`;
	if (summary.rate_limited > 0) return `${summary.rate_limited} rate limited`;
	if (summary.active > 0) return `${summary.active} active`;
	return `${summary.disabled} disabled`;
}

// The provider label used in a heading or a table cell. `label` is reused so a name is bounded the same way
// everywhere; the fallback keeps a minimal registry entry from rendering a blank row.
export const providerLabel = label;
