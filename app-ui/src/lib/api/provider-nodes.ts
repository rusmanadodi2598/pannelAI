// Custom provider node calls, mirroring docs/SPEC-API/001-SPEC-API.md §7.4.
//
// Six routes over one resource: list, create, read, patch, delete, and the connectivity test. The list
// carries no query the panel needs (a node set is small enough that §7.4 returns all of it and takes no
// paging), and only the type filter exists, which the screen does not offer because the section already
// shows both types together.
//
// The test route is here rather than beside the endpoint test because its path is the node's: §7.4
// nests it under `/provider-nodes/{id}`, and a node carries no credential of its own, so the body is
// optional and the panel sends one only when the operator typed it.

import {
	schemaProviderNode,
	schemaProviderNodeList,
	schemaProviderNodeProbe,
	type ProviderNode,
	type ProviderNodeList,
	type ProviderNodeProbe
} from '$lib/schemas/provider-node';
import type {
	CreateProviderNodeBody,
	UpdateProviderNodeBody
} from '$lib/schemas/provider-node-draft';
import { emptyResponse, type EmptyResponse } from '$lib/schemas/primitives';
import { apiRequest, type ApiResult } from './client';

/** Every node, newest state the gateway holds. Not paginated, so the answer is the whole set. */
export function listProviderNodes(): Promise<ApiResult<ProviderNodeList>> {
	return apiRequest<void, ProviderNodeList>({
		method: 'GET',
		path: '/provider-nodes',
		schema: schemaProviderNodeList
	});
}

/**
 * Creates one node.
 *
 * A prefix that would shadow a registry identifier or another node is refused as CONFLICT, and the
 * panel does not predict that answer: it renders the gateway's own message, which names the owner.
 */
export function createProviderNode(body: CreateProviderNodeBody): Promise<ApiResult<ProviderNode>> {
	return apiRequest<CreateProviderNodeBody, ProviderNode>({
		method: 'POST',
		path: '/provider-nodes',
		schema: schemaProviderNode,
		body
	});
}

/**
 * One node's stored shape.
 *
 * The provider read is not enough on this screen: the gateway synthesizes a provider from the node and
 * that entry carries the base URL and the wire format, but not the prefix, the api type, or when the
 * node was created. Those are what the edit form starts from and what the card states.
 */
export function getProviderNode(id: string): Promise<ApiResult<ProviderNode>> {
	return apiRequest<void, ProviderNode>({
		method: 'GET',
		path: `/provider-nodes/${encodeURIComponent(id)}`,
		schema: schemaProviderNode
	});
}

/** Patches a node's name, prefix, or base URL. Type and api type are not patchable (§7.4). */
export function updateProviderNode(
	id: string,
	body: UpdateProviderNodeBody
): Promise<ApiResult<ProviderNode>> {
	return apiRequest<UpdateProviderNodeBody, ProviderNode>({
		method: 'PATCH',
		path: `/provider-nodes/${encodeURIComponent(id)}`,
		schema: schemaProviderNode,
		body
	});
}

/**
 * Removes a node. The route answers 204 with no body.
 *
 * Refused as CONFLICT while an endpoint still references the node, because deleting it would leave that
 * endpoint routing to a provider that no longer exists.
 */
export function deleteProviderNode(id: string): Promise<ApiResult<EmptyResponse>> {
	return apiRequest<void, EmptyResponse>({
		method: 'DELETE',
		path: `/provider-nodes/${encodeURIComponent(id)}`,
		schema: emptyResponse
	});
}

/**
 * Probes the node's base URL and reports whether it answers.
 *
 * A refused credential answers 200 with a fail state rather than an error, so the result is rendered
 * rather than thrown. `credential` is omitted rather than sent empty when the operator typed none: an
 * absent value legitimately tests a node whose upstream needs no credential.
 */
export function testProviderNode(
	id: string,
	credential?: string
): Promise<ApiResult<ProviderNodeProbe>> {
	return apiRequest<{ credential?: string }, ProviderNodeProbe>({
		method: 'POST',
		path: `/provider-nodes/${encodeURIComponent(id)}/test`,
		schema: schemaProviderNodeProbe,
		body: credential === undefined || credential === '' ? {} : { credential }
	});
}
