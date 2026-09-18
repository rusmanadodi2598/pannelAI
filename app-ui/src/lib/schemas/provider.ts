// Provider registry schemas, mirroring docs/SPEC-API/001-SPEC-API.md §7.4 and the table in
// docs/SPEC-UI/001-SPEC-UI.md §6.3.
//
// `category`, `routability`, and `kind` are plain strings rather than enums. The registry treats category
// as an open string, so a panel that enumerated it would fail to render a provider the API had just
// returned; the panel filters by the values it sees and shows the rest verbatim. §14 Q9 records the same
// decision for a gateway key's status.
//
// The provider's models are read from the provider-scoped route, which is the one that carries `suggested`
// and `capabilities`. The global catalog (`/models/catalog`) reports neither, so it cannot answer the
// "suggested" toggle §6.3 asks for.

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
export const schemaProviderDetail = schemaProvider.extend({
	base_url: z.string(),
	format: z.string(),
	url_suffix: z.string(),
	validate_url: z.string(),
	timeout_ms: z.number().int(),
	model_count: z.number().int().min(0),
	chat_model_count: z.number().int().min(0),
	media: z.array(schemaProviderMedia).nullish().transform((value) => value ?? []),
	deprecated: z.boolean(),
	deprecation_notice: z.string().optional(),
	website: z.string().optional()
});

export type ProviderDetail = z.infer<typeof schemaProviderDetail>;

// One model inside a provider. `kind` is carried because a non-chat model is not routable through the chat
// data plane, so hiding it would offer a model string that cannot answer.
export const schemaProviderModel = z.object({
	id: z.string().min(1),
	name: z.string(),
	kind: z.string(),
	capabilities: stringList,
	dimensions: z.number().int().optional(),
	suggested: z.boolean()
});

export type ProviderModel = z.infer<typeof schemaProviderModel>;

export const schemaProviderModelList = z.object({
	data: z.array(schemaProviderModel)
});

export type ProviderModelList = z.infer<typeof schemaProviderModelList>;

// The categories the reference registry measures. Offered as filter suggestions, not as a closed set: the
// list the API returns is authoritative, and a provider with a category outside this list still renders.
export const PROVIDER_CATEGORIES = ['apikey', 'oauth', 'free', 'media', 'local'] as const;

// The two capability filters §6.3 asks for, matched case-insensitively against a model's capability list.
export const MODEL_CAPABILITY_FILTERS = ['vision', 'tools'] as const;

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

// A model's capabilities as the panel shows them. Empty is legitimate and renders as no chips rather than
// as a placeholder, because a model with no declared capability is a real state.
export function hasCapability(model: ProviderModel, capability: string): boolean {
	const wanted = capability.toLowerCase();
	return model.capabilities.some((entry) => entry.toLowerCase() === wanted);
}

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
