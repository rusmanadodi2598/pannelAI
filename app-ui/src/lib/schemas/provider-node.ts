// Custom provider node schemas, mirroring docs/SPEC-API/001-SPEC-API.md §7.4 and the Custom provider
// section of docs/SPEC-UI/001-SPEC-UI.md §6.3.
//
// A node is a base URL the embedded registry does not ship. The gateway synthesizes a provider entry
// from it, so a node becomes routable exactly like a registry provider, and the panel never has to
// branch on "is this custom" downstream: it reads the same provider shapes either way.
//
// This module is the read side plus the derivations a screen shows. Every shape the panel sends, and the
// form copy that goes with it, lives in `./provider-node-draft`.

import { z } from 'zod';
import { optionalTimestamp } from './primitives';

/**
 * The two node id prefixes. They are part of the public contract (§7.4): a node's id appears in
 * endpoint rows and in the panel's own URLs, and the registry derives the node's wire format from
 * which prefix it carries. The provider response carries no `custom` flag, so this is how the panel
 * tells a node from a registry provider.
 */
export const NODE_ID_PREFIXES = {
	'openai-compatible': 'openai-compatible-',
	'anthropic-compatible': 'anthropic-compatible-'
} as const;

export const NODE_TYPES = ['openai-compatible', 'anthropic-compatible'] as const;
export type NodeType = (typeof NODE_TYPES)[number];

export const NODE_TYPE_LABELS: Record<NodeType, string> = {
	'openai-compatible': 'OpenAI Compatible',
	'anthropic-compatible': 'Anthropic Compatible'
};

/**
 * The wire format an OpenAI-compatible node speaks. §7.4 requires one of the two, and refuses the
 * field on an Anthropic-compatible node because its single endpoint has no such distinction.
 */
export const NODE_API_TYPES = ['chat', 'responses'] as const;
export type NodeAPIType = (typeof NODE_API_TYPES)[number];

export const NODE_API_TYPE_LABELS: Record<NodeAPIType, string> = {
	chat: 'Chat Completions',
	responses: 'Responses API'
};

// `type`, `api_type`, and `format` are plain strings rather than enums, for the reason `provider.ts`
// gives for `category`: the API owns the vocabulary, and a panel that closed it would fail to render a
// node the API had just returned.
export const schemaProviderNode = z.object({
	id: z.string().min(1),
	type: z.string().min(1),
	name: z.string().min(1),
	prefix: z.string().min(1),
	api_type: z.string().optional(),
	base_url: z.string(),
	format: z.string(),
	created_at: z.string(),
	updated_at: z.string()
});

export type ProviderNode = z.infer<typeof schemaProviderNode>;

// The list carries no meta block: §7.4 returns every node, because nodes are a hand-configured set
// rather than a table that grows with traffic.
export const schemaProviderNodeList = z.object({ data: z.array(schemaProviderNode) });

export type ProviderNodeList = z.infer<typeof schemaProviderNodeList>;

// The connectivity result. `state` stays a string so a new prober state renders rather than breaks,
// and a refused credential answers 200 with a fail state rather than an error.
export const schemaProviderNodeProbe = z.object({
	state: z.string().min(1),
	latency_ms: z.number().int(),
	message: z.string().optional(),
	checked_at: optionalTimestamp
});

export type ProviderNodeProbe = z.infer<typeof schemaProviderNodeProbe>;

// The two members a probe outcome has (`domain.EndpointTestOK` / `EndpointTestFail`, the pair the proxy
// probe reports too). A state the panel does not know is returned as it arrived, which is why this is a
// lookup with a fallback rather than an enum in the response schema above.
export const NODE_TEST_STATE_LABELS: Record<string, string> = {
	ok: 'Reachable',
	fail: 'Failed'
};

export function nodeTestStateLabel(state: string): string {
	return NODE_TEST_STATE_LABELS[state] ?? state;
}

/** The node type an id carries, or null when the id is a registry provider's. */
export function nodeTypeOfId(id: string): NodeType | null {
	for (const type of NODE_TYPES) {
		if (id.startsWith(NODE_ID_PREFIXES[type])) return type;
	}
	return null;
}

/** True when the provider id names a custom node rather than a registry entry. */
export function isNodeId(id: string): boolean {
	return nodeTypeOfId(id) !== null;
}

/**
 * The path the gateway appends to the node's base URL.
 *
 * Derived here rather than read from the node, because the node response reports `format` (which wire
 * dialect it speaks) and not the path. The three values are the registry's own (`custom_node.go`
 * `chatPath`), so the sentence this screen shows is the request the gateway will make.
 */
export function nodeEndpointPath(node: Pick<ProviderNode, 'type' | 'api_type'>): string {
	if (node.type === 'anthropic-compatible') return '/messages';
	return node.api_type === 'responses' ? '/responses' : '/chat/completions';
}

/** The name of the endpoint that path reaches, which is what the card labels the node with. */
export function nodeEndpointLabel(node: Pick<ProviderNode, 'type' | 'api_type'>): string {
	if (node.type === 'anthropic-compatible') return 'Messages API';
	return node.api_type === 'responses' ? 'Responses API' : 'Chat Completions';
}

/** The full URL the gateway will call for this node, with exactly one separator before the path. */
export function nodeEndpointUrl(
	node: Pick<ProviderNode, 'type' | 'api_type' | 'base_url'>
): string {
	return `${node.base_url.replace(/\/+$/, '')}${nodeEndpointPath(node)}`;
}
