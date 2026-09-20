// Proxy pool calls, mirroring docs/SPEC-API/001-SPEC-API.md §7.11.
//
// Every path is the one the API publishes, and every response goes through the schema for its shape,
// so a field the API renames fails the parse here rather than surfacing as `undefined` in a table
// cell.
//
// The two test routes both answer 200 with a result, including when the proxy refused the
// connection: a failed probe is the answer the operator asked for, not an error. That is why both
// return a `ProxyTest` rather than a thrown failure, and why the screen renders a fail state instead
// of an error banner.
//
// A create and a patch carry the same body, so they share `schemaProxyForm`. On a patch an empty
// password is the message that keeps the stored secret; the panel never holds the value to send
// back, because no response carries it.

import {
	schemaProxy,
	schemaProxyCandidate,
	schemaProxyForm,
	schemaProxyList,
	schemaProxyTest,
	type Proxy,
	type ProxyCandidate,
	type ProxyForm,
	type ProxyList,
	type ProxyTest
} from '$lib/schemas/proxy';
import { emptyResponse, type EmptyResponse } from '$lib/schemas/primitives';
import { apiRequest, type ApiResult } from './client';

// §7.11 lists the pool without pagination: the whole set comes back in one page.
export function listProxies(): Promise<ApiResult<ProxyList>> {
	return apiRequest<void, ProxyList>({
		method: 'GET',
		path: '/proxies',
		schema: schemaProxyList
	});
}

export function createProxy(form: ProxyForm): Promise<ApiResult<Proxy>> {
	return apiRequest<ProxyForm, Proxy>({
		method: 'POST',
		path: '/proxies',
		schema: schemaProxy,
		body: form,
		bodySchema: schemaProxyForm
	});
}

export function updateProxy(id: string, form: ProxyForm): Promise<ApiResult<Proxy>> {
	return apiRequest<ProxyForm, Proxy>({
		method: 'PATCH',
		path: `/proxies/${encodeURIComponent(id)}`,
		schema: schemaProxy,
		body: form,
		bodySchema: schemaProxyForm
	});
}

export function deleteProxy(id: string): Promise<ApiResult<EmptyResponse>> {
	return apiRequest<void, EmptyResponse>({
		method: 'DELETE',
		path: `/proxies/${encodeURIComponent(id)}`,
		schema: emptyResponse
	});
}

// Probes the stored candidate and stores what it found, so the list and this answer agree.
export function testProxy(id: string): Promise<ApiResult<ProxyTest>> {
	return apiRequest<void, ProxyTest>({
		method: 'POST',
		path: `/proxies/${encodeURIComponent(id)}/test`,
		schema: schemaProxyTest
	});
}

// Probes an unsaved candidate and stores nothing, which is what the form uses before a save.
export function testProxyCandidate(candidate: ProxyCandidate): Promise<ApiResult<ProxyTest>> {
	return apiRequest<ProxyCandidate, ProxyTest>({
		method: 'POST',
		path: '/proxies/test',
		schema: schemaProxyTest,
		body: candidate,
		bodySchema: schemaProxyCandidate
	});
}
