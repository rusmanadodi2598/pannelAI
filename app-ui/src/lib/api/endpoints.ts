// Upstream endpoint and key calls, mirroring docs/SPEC-API/001-SPEC-API.md §7.5.
//
// Every path is the one the API publishes, and every response goes through the schema for its shape, so a
// field the API renames fails the parse here rather than surfacing as `undefined` in a table cell.

import {
	schemaEndpoint,
	schemaEndpointKey,
	schemaEndpointKeyList,
	schemaEndpointLabelList,
	schemaEndpointList,
	schemaEndpointTestStatus,
	type Endpoint,
	type EndpointKey,
	type EndpointKeyList,
	type EndpointLabelList,
	type EndpointList,
	type EndpointTestStatus
} from '$lib/schemas/endpoint';
import {
	createEndpointBody,
	schemaAddEndpointKeyForm,
	schemaBulkAddKeysForm,
	schemaBulkCreateEndpointsBody,
	schemaCreateEndpointBody,
	schemaUpdateEndpointForm,
	schemaUpdateEndpointKeyForm,
	type AddEndpointKeyForm,
	type BulkAddKeysForm,
	type BulkCreateEndpointsBody,
	type CreateEndpointBody,
	type CreateEndpointForm,
	type UpdateEndpointForm,
	type UpdateEndpointKeyForm
} from '$lib/schemas/endpoint-write';
import {
	schemaBulkEndpointResult,
	schemaBulkKeyResult,
	schemaBulkRefusal,
	type BulkEndpointResult,
	type BulkKeyResult,
	type BulkRefusal
} from '$lib/schemas/endpoint-bulk';
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

// The label-only read screens that name endpoints beside other data use (§6.6). Same route as the list,
// one projection fewer fields, so a screen that only names rows cannot fail on a field it never reads. It
// takes the list's own filters, because the provider detail screen reads the labels of one provider.
export function listEndpointLabels(
	query: EndpointQuery = {}
): Promise<ApiResult<EndpointLabelList>> {
	return apiRequest<void, EndpointLabelList>({
		method: 'GET',
		path: '/endpoints',
		schema: schemaEndpointLabelList,
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

// The form is a screen's shape and the body is the route's, so the mapping is explicit: `keys` is what
// `POST /endpoints` reads, and a body carrying a form-only field is refused by the decoder
// (`DisallowUnknownFields`). Every create path goes through `createEndpointBody`.
export function createEndpoint(form: CreateEndpointForm): Promise<ApiResult<Endpoint>> {
	return apiRequest<CreateEndpointBody, Endpoint>({
		method: 'POST',
		path: '/endpoints',
		schema: schemaEndpoint,
		body: createEndpointBody(form),
		bodySchema: schemaCreateEndpointBody
	});
}

// Several connections at one provider in one call (§7.5). The batch is all-or-nothing, so the accepted
// answer carries the created connections and the refusal names every row by index with the message on the
// offending one; the refusal is parsed because it is what lets a screen name the pasted line to fix.
export function createEndpointsBulk(
	body: BulkCreateEndpointsBody
): Promise<ApiResult<BulkEndpointResult, BulkRefusal>> {
	return apiRequest<BulkCreateEndpointsBody, BulkEndpointResult, BulkRefusal>({
		method: 'POST',
		path: '/endpoints/bulk',
		schema: schemaBulkEndpointResult,
		refusalSchema: schemaBulkRefusal,
		body,
		bodySchema: schemaBulkCreateEndpointsBody
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
// every row by index and `created` is empty on a refusal (§8.1). The refusal is parsed too: it is what
// lets a refused row keep the server's own message instead of the screen guessing which row to fix.
export function addEndpointKeys(
	id: string,
	form: BulkAddKeysForm
): Promise<ApiResult<BulkKeyResult, BulkRefusal>> {
	return apiRequest<BulkAddKeysForm, BulkKeyResult, BulkRefusal>({
		method: 'POST',
		path: `/endpoints/${encodeURIComponent(id)}/keys/bulk`,
		schema: schemaBulkKeyResult,
		refusalSchema: schemaBulkRefusal,
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
