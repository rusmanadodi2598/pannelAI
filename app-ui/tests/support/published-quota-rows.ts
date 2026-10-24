// The published-quota read's wire rows (docs/SPEC-API/001-SPEC-API.md §7.12), loose on purpose: the cases
// below feed values the API itself would refuse, which a typed fixture cannot express.

import {
	schemaPublishedQuotaUsage,
	schemaPublishedQuotaWindow,
	type PublishedQuotaUsage,
	type PublishedQuotaWindow
} from '$lib/schemas/quota-published';

export function publishedWindowRow(
	overrides: Record<string, unknown> = {}
): Record<string, unknown> {
	return { label: 'Personal', used: '12.5', total: '3000', ...overrides };
}

export function publishedUsageRow(
	overrides: Record<string, unknown> = {}
): Record<string, unknown> {
	return {
		endpoint_id: 'ep_1',
		provider_id: 'qoder',
		plan: 'personal_standard',
		fetched_at: '2026-09-28T12:00:00Z',
		data: [publishedWindowRow()],
		...overrides
	};
}

export function parsePublishedWindow(row: Record<string, unknown>): PublishedQuotaWindow {
	return schemaPublishedQuotaWindow.parse(row);
}

export function parsePublishedUsage(body: Record<string, unknown>): PublishedQuotaUsage {
	return schemaPublishedQuotaUsage.parse(body);
}
