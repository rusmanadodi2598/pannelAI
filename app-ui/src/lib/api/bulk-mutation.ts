// The bulk mutation routes (docs/SPEC-API/001-SPEC-API.md §7.5, §8.1, SPEC-UI §6.2).
//
// Eight POSTs share two shapes: a body of `{selector, ...}` and an answer of `{matched, changed, results}`.
// They are declared together because the selector rule that makes them safe is one rule, and a screen that
// forgot `confirm_count` would be erasing rows the operator never counted. `confirm_count` is the number
// the panel displayed: the server resolves the selector, counts the rows, and refuses when the two differ.

import { apiRequest, type ApiResult } from './client';
import {
	bulkSelectorBody,
	schemaBulkMutationResult,
	schemaBulkProxyResult,
	type BulkMutationResult,
	type BulkProxyResult,
	type BulkSelector
} from '$lib/schemas/bulk-mutation';

/** The row set a bulk write resolves: a provider's connection list, or one connection's keys. */
export type BulkKind = 'endpoints' | 'endpoint-keys';

/** The rows the panel displayed, echoed back so the server can refuse a set that changed underneath them. */
interface BulkEnvelope {
	selector: BulkSelector;
	confirm_count: number;
}

function bulkBody(
	selector: BulkSelector,
	confirmCount: number,
	extra: Record<string, unknown>
): BulkEnvelope {
	return {
		selector: bulkSelectorBody(selector),
		confirm_count: confirmCount,
		...extra
	};
}

function postBulk(
	kind: BulkKind,
	action: string,
	body: BulkEnvelope
): Promise<ApiResult<BulkMutationResult>> {
	return apiRequest<BulkEnvelope, BulkMutationResult>({
		method: 'POST',
		path: `/${kind}/bulk-${action}`,
		schema: schemaBulkMutationResult,
		body
	});
}

/** Park or release the resolved set. `windowHours` is omitted to take the provider's own reset cadence. */
export function bulkStatus(
	kind: BulkKind,
	selector: BulkSelector,
	status: 'active' | 'disabled',
	confirmCount: number,
	windowHours?: number
): Promise<ApiResult<BulkMutationResult>> {
	return postBulk(
		kind,
		'status',
		bulkBody(selector, confirmCount, {
			status,
			...(windowHours ? { window_hours: windowHours } : {})
		})
	);
}

/** Add, remove, or clear tags across the resolved set. */
export function bulkTags(
	kind: BulkKind,
	selector: BulkSelector,
	add: string[],
	remove: string[],
	clear: boolean,
	confirmCount: number
): Promise<ApiResult<BulkMutationResult>> {
	return postBulk(
		kind,
		'tags',
		bulkBody(selector, confirmCount, {
			...(add.length ? { add } : {}),
			...(remove.length ? { remove } : {}),
			...(clear ? { clear: true } : {})
		})
	);
}

/** Erase the resolved set, permanently. */
export function bulkDelete(
	kind: BulkKind,
	selector: BulkSelector,
	confirmCount: number
): Promise<ApiResult<BulkMutationResult>> {
	return postBulk(kind, 'delete', bulkBody(selector, confirmCount, {}));
}

/** Switch every selected candidate on or off, leaving each one's probe verdict where it is. */
export function bulkProxyStatus(
	selector: BulkSelector,
	enabled: boolean,
	confirmCount: number
): Promise<ApiResult<BulkProxyResult>> {
	return apiRequest<BulkEnvelope, BulkProxyResult>({
		method: 'POST',
		path: '/proxies/bulk-status',
		schema: schemaBulkProxyResult,
		body: bulkBody(selector, confirmCount, { enabled })
	});
}

/**
 * Ask what an erase would take down, without erasing anything. The panel needs the number before it can
 * ask the operator to confirm it, and only storage knows how many connections hang on a candidate.
 */
export function bulkProxyPreview(
	selector: BulkSelector,
	confirmCount: number
): Promise<ApiResult<BulkProxyResult>> {
	return apiRequest<BulkEnvelope, BulkProxyResult>({
		method: 'POST',
		path: '/proxies/bulk-preview',
		schema: schemaBulkProxyResult,
		body: bulkBody(selector, confirmCount, { confirm_endpoints: 0 })
	});
}

/**
 * Erase every selected candidate. `confirmEndpoints` is the number of connections the operator saw lose
 * their egress path: the pool is pinned by an id string, so this is the only thing that makes the cost
 * of the erase visible before it happens.
 */
export function bulkProxyDelete(
	selector: BulkSelector,
	confirmCount: number,
	confirmEndpoints: number
): Promise<ApiResult<BulkProxyResult>> {
	return apiRequest<BulkEnvelope, BulkProxyResult>({
		method: 'POST',
		path: '/proxies/bulk-delete',
		schema: schemaBulkProxyResult,
		body: bulkBody(selector, confirmCount, { confirm_endpoints: confirmEndpoints })
	});
}
