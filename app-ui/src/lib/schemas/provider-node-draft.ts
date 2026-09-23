// The write side of a custom provider node: the add and edit form's copy, its rules, and the bodies a
// save carries (docs/SPEC-API/001-SPEC-API.md §7.4, docs/SPEC-UI/001-SPEC-UI.md §6.3).
//
// Split from `./provider-node` the way `./endpoint-write` is split from `./endpoint`: the read shapes and
// the write shapes are different jobs, and the file holding both crosses the panel's line limit. The
// reference keeps the same copy per variant in one table (`AddCompatibleModal.js:7-30`), which is what
// `NODE_COPY` is: one entry per type, so the two dialogs cannot drift apart.
//
// Two of the rules here are the API's own and are restated rather than invented, because a save the panel
// blocks should be one the gateway would have refused: a prefix is a model-string namespace
// (`prefix/model`), so it carries letters, digits, dots, dashes, and underscores and nothing else, and
// `api_type` belongs to an OpenAI-compatible node alone.

import { z } from 'zod';
import { absoluteUrl, label } from './primitives';
import {
	NODE_API_TYPES,
	type NodeAPIType,
	type NodeType,
	type ProviderNode
} from './provider-node';

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

/**
 * The vendor URL each type is pointed at by default. It is the field's value, not its placeholder: the
 * reference pre-fills its base URL the same way (`AddCompatibleModal.js:11`, `:22`), so the common case is
 * a save with no typing, and a typed URL that is one segment off is still visible against the default.
 */
export const NODE_BASE_URL_DEFAULTS: Record<NodeType, string> = {
	'openai-compatible': 'https://api.openai.com/v1',
	'anthropic-compatible': 'https://api.anthropic.com/v1'
};

/**
 * The copy the add dialog states per type, taken from the reference's own table so the two variants cannot
 * end up describing the same rule in two different ways.
 */
export const NODE_COPY: Record<
	NodeType,
	{ namePlaceholder: string; prefixPlaceholder: string; baseUrlHint: string }
> = {
	'openai-compatible': {
		namePlaceholder: 'OpenAI Compatible (Prod)',
		prefixPlaceholder: 'oc-prod',
		baseUrlHint: 'Use the base URL (ending in /v1) for your OpenAI-compatible API.'
	},
	'anthropic-compatible': {
		namePlaceholder: 'Anthropic Compatible (Prod)',
		prefixPlaceholder: 'ac-prod',
		baseUrlHint:
			'Use the base URL (ending in /v1) for your Anthropic-compatible API. The gateway appends /messages.'
	}
};

/** A draft for a new node of `type`, with the vendor URL already in the field. */
export function customProviderDraftNew(type: NodeType): CustomProviderDraft {
	return { name: '', prefix: '', api_type: 'chat', base_url: NODE_BASE_URL_DEFAULTS[type] };
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
