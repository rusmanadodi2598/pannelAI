// Upstream endpoint and key calls, mirroring docs/SPEC-API/001-SPEC-API.md §7.5.
//
// Every path is the one the API publishes, and every response goes through the schema for its shape, so a
// field the API renames fails the parse here rather than surfacing as `undefined` in a table cell.

import {
	schemaAddEndpointKeyForm,
	schemaBulkAddKeysForm,
	schemaBulkKeyResult,
	schemaCreateEndpointForm,
	schemaEndpoint,
	schemaEndpointKey,
	schemaEndpointKeyList,
	schemaEndpointList,
	schemaEndpointTestStatus,
	schemaUpdateEndpointForm,
	schemaUpdateEndpointKeyForm,
	type AddEndpointKeyForm,
	type BulkAddKeysForm,
	type BulkKeyResult,
	type CreateEndpointForm,
	type Endpoint,
	type EndpointKey,
	type EndpointKeyList,
	type EndpointList,
	type EndpointTestStatus,
	type UpdateEndpointForm,
	type UpdateEndpointKeyForm
} from '$lib/schemas/endpoint';
import { emptyResponse, type EmptyResponse } from '$lib/schemas/primitives';
import { apiRequest, type ApiResult } from './client';

export type ListQuery = {
	page?: number;
	per_page?: number;
};

// The list filters §6.2 asks for. `provider_id` and `status` are the two the table offers.
export type EndpointQuery = ListQuery & {
	provider_id?: string;
	status?: string;
};

export function listEndpoints(query: EndpointQuery = {}): Promise<ApiResult<EndpointList>> {
	return apiRequest<void, EndpointList>({
		method: 'GET',
		path: '/endpoints',
		schema: schemaEndpointList,
		query
	});
}

// Detail carries the keys, which the list shape does not.
export function getEndpoint(id: string): Promise<ApiResult<Endpoint>> {
	return apiRequest<void, Endpoint>({
		method: 'GET',
		path: `/endpoints/${encodeURIComponent(id)}`,
		schema: schemaEndpoint
	});
}

export function createEndpoint(form: CreateEndpointForm): Promise<ApiResult<Endpoint>> {
	return apiRequest<CreateEndpointForm, Endpoint>({
		method: 'POST',
		path: '/endpoints',
		schema: schemaEndpoint,
		body: form,
		bodySchema: schemaCreateEndpointForm
	});
}

// Label, priority, and status all answer with the updated endpoint, so one schema covers them. A priority
// change reorders siblings, which is why §6.2 refreshes the list after a successful save.
export function updateEndpoint(id: string, form: UpdateEndpointForm): Promise<ApiResult<Endpoint>> {
	return apiRequest<UpdateEndpointForm, Endpoint>({
		method: 'PATCH',
		path: `/endpoints/${encodeURIComponent(id)}`,
		schema: schemaEndpoint,
		body: form,
		bodySchema: schemaUpdateEndpointForm
	});
}

// The API deletes the endpoint's keys with it, which the confirmation states.
export function deleteEndpoint(id: string): Promise<ApiResult<EmptyResponse>> {
	return apiRequest<void, EmptyResponse>({
		method: 'DELETE',
		path: `/endpoints/${encodeURIComponent(id)}`,
		schema: emptyResponse
	});
}

export function listEndpointKeys(
	id: string,
	query: ListQuery = {}
): Promise<ApiResult<EndpointKeyList>> {
	return apiRequest<void, EndpointKeyList>({
		method: 'GET',
		path: `/endpoints/${encodeURIComponent(id)}/keys`,
		schema: schemaEndpointKeyList,
		query
	});
}

export function addEndpointKey(
	id: string,
	form: AddEndpointKeyForm
): Promise<ApiResult<EndpointKey>> {
	return apiRequest<AddEndpointKeyForm, EndpointKey>({
		method: 'POST',
		path: `/endpoints/${encodeURIComponent(id)}/keys`,
		schema: schemaEndpointKey,
		body: form,
		bodySchema: schemaAddEndpointKeyForm
	});
}

// The repeatable row mode's single request (§6.2). The batch is all-or-nothing, so the result reports
// every row by index and `created` is empty on a refusal (§8.1).
export function addEndpointKeys(
	id: string,
	form: BulkAddKeysForm
): Promise<ApiResult<BulkKeyResult>> {
	return apiRequest<BulkAddKeysForm, BulkKeyResult>({
		method: 'POST',
		path: `/endpoints/${encodeURIComponent(id)}/keys/bulk`,
		schema: schemaBulkKeyResult,
		body: form,
		bodySchema: schemaBulkAddKeysForm
	});
}

// An omitted `value` keeps the stored credential, because the value is write-only on the wire.
export function updateEndpointKey(
	id: string,
	keyId: string,
	form: UpdateEndpointKeyForm
): Promise<ApiResult<EndpointKey>> {
	return apiRequest<UpdateEndpointKeyForm, EndpointKey>({
		method: 'PATCH',
		path: `/endpoints/${encodeURIComponent(id)}/keys/${encodeURIComponent(keyId)}`,
		schema: schemaEndpointKey,
		body: form,
		bodySchema: schemaUpdateEndpointKeyForm
	});
}

// The API answers CONFLICT when this would leave an api_key endpoint with nothing to route with (§7.5).
export function deleteEndpointKey(id: string, keyId: string): Promise<ApiResult<EmptyResponse>> {
	return apiRequest<void, EmptyResponse>({
		method: 'DELETE',
		path: `/endpoints/${encodeURIComponent(id)}/keys/${encodeURIComponent(keyId)}`,
		schema: emptyResponse
	});
}

// A refused credential answers 200 with a fail state rather than an error, so the result is rendered
// rather than thrown. An absent key_id tests the first active key, which is what routing would spend.
export function testEndpoint(id: string, keyId?: string): Promise<ApiResult<EndpointTestStatus>> {
	return apiRequest<{ key_id?: string }, EndpointTestStatus>({
		method: 'POST',
		path: `/endpoints/${encodeURIComponent(id)}/test`,
		schema: schemaEndpointTestStatus,
		body: keyId === undefined ? {} : { key_id: keyId }
	});
}
