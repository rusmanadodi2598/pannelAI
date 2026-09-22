// Custom provider node schemas, mirroring docs/SPEC-API/001-SPEC-API.md §7.4 and the Custom provider
// section of docs/SPEC-UI/001-SPEC-UI.md §6.3.
//
// A node is a base URL the embedded registry does not ship. The gateway synthesizes a provider entry
// from it, so a node becomes routable exactly like a registry provider, and the panel never has to
// branch on "is this custom" downstream: it reads the same provider shapes either way.
//
// Two of the rules here are the API's own and are restated rather than invented, because a save the
// panel blocks should be one the gateway would have refused: a prefix is a model-string namespace
// (`prefix/model`), so it carries letters, digits, dots, dashes, and underscores and nothing else, and
// `api_type` belongs to an OpenAI-compatible node alone.

import { z } from 'zod';
import { absoluteUrl, label, optionalTimestamp } from './primitives';

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

// A model-string namespace. The API's message is mirrored so the panel and the gateway describe the
// same rule the same way; `/` and whitespace are the values that make `prefix/model` unparseable.
const nodePrefix = z
	.string()
	.transform((value) => value.trim())
	.refine((value) => value.length > 0, { message: 'A prefix is required.' })
	.refine((value) => value.length <= 64, { message: 'Use 64 characters or fewer.' })
	.refine((value) => /^[A-Za-z0-9._-]+$/.test(value), {
		message: 'Use letters, digits, dots, dashes, and underscores only.'
	});

// A trailing slash is dropped rather than kept: the gateway joins the path with one separator
// (`provider.joinPath`), so a base ending in `/` would send `//chat/completions` to an upstream that
// may treat it as a different path. `absoluteUrl` already strips one; this strips a run.
const nodeBaseURL = absoluteUrl.transform((value) => value.replace(/\/+$/, ''));

export const schemaCustomProviderDraft = z.strictObject({
	name: label,
	prefix: nodePrefix,
	api_type: z.enum(NODE_API_TYPES),
	base_url: nodeBaseURL
});

export type CustomProviderDraft = z.infer<typeof schemaCustomProviderDraft>;

/** The vendor URL each type is most often pointed at, used as the field's placeholder, not its value. */
export const NODE_BASE_URL_HINTS: Record<NodeType, string> = {
	'openai-compatible': 'https://api.openai.com/v1',
	'anthropic-compatible': 'https://api.anthropic.com/v1'
};

/** A blank draft for a new node of `type`. */
export function customProviderDraftNew(): CustomProviderDraft {
	return { name: '', prefix: '', api_type: 'chat', base_url: '' };
}

/** The draft seeded from a stored node, so an edit starts from what is stored rather than from blank. */
export function customProviderDraftFrom(node: ProviderNode): CustomProviderDraft {
	return {
		name: node.name,
		prefix: node.prefix,
		api_type: node.api_type === 'responses' ? 'responses' : 'chat',
		base_url: node.base_url
	};
}

export type CreateProviderNodeBody = {
	name: string;
	prefix: string;
	type: NodeType;
	api_type?: NodeAPIType;
	base_url: string;
};

/**
 * The create body for `type`.
 *
 * `api_type` is sent for an OpenAI-compatible node only: §7.4 refuses the field on an
 * Anthropic-compatible node, so carrying it would make every Anthropic create fail.
 */
export function createProviderNodeBody(
	type: NodeType,
	draft: CustomProviderDraft
): CreateProviderNodeBody {
	const body: CreateProviderNodeBody = {
		name: draft.name,
		prefix: draft.prefix,
		type,
		base_url: draft.base_url
	};
	if (type === 'openai-compatible') body.api_type = draft.api_type;
	return body;
}

export type UpdateProviderNodeBody = { name: string; prefix: string; base_url: string };

/**
 * The patch body. `type` and `api_type` are absent because they decide which wire format the node
 * speaks and are part of its identity: §7.4 makes changing one a new node, so the edit form shows the
 * endpoint as a fact rather than as a control that cannot act.
 */
export function updateProviderNodeBody(draft: CustomProviderDraft): UpdateProviderNodeBody {
	return { name: draft.name, prefix: draft.prefix, base_url: draft.base_url };
}
