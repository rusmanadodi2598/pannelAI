// Tests for the custom provider node contracts (docs/SPEC-API/001-SPEC-API.md §7.4,
// docs/SPEC-UI/001-SPEC-UI.md §6.3).
//
// What a stored node means: how its id names its type, which endpoint the gateway appends to its base
// URL, what the list envelope looks like, and how a probe state is labelled. All of it is derived from
// the wire, so the fixture here is typed and every case is a value the route can actually send.
//
// The form's draft rules and the bodies a write carries are in `provider-node-draft.test.ts`; this file
// never builds a request body.

import { describe, expect, it } from 'vitest';
import {
	isNodeId,
	nodeEndpointLabel,
	nodeEndpointPath,
	nodeEndpointUrl,
	nodeTestStateLabel,
	nodeTypeOfId,
	schemaProviderNode,
	schemaProviderNodeList,
	type ProviderNode
} from '$lib/schemas/provider-node';

const node = (overrides: Partial<ProviderNode> = {}): ProviderNode => ({
	id: 'openai-compatible-01J',
	type: 'openai-compatible',
	name: 'OpenAI Compatible (prod)',
	prefix: 'mycorp',
	api_type: 'chat',
	base_url: 'https://llm.example.com/v1',
	format: 'openai',
	created_at: '2026-09-22T03:00:00Z',
	updated_at: '2026-09-22T04:00:00Z',
	...overrides
});

describe('node ids', () => {
	it.each([
		{
			label: 'an OpenAI-compatible node',
			id: 'openai-compatible-01J',
			expected: 'openai-compatible'
		},
		{
			label: 'an Anthropic-compatible node',
			id: 'anthropic-compatible-01J',
			expected: 'anthropic-compatible'
		},
		{ label: 'a registry provider', id: 'openai', expected: null },
		{
			label: 'a registry provider whose id starts with a word',
			id: 'openai-compat',
			expected: null
		},
		{ label: 'an empty id', id: '', expected: null },
		{ label: 'a hidden registry entry', id: 'codebuddy', expected: null }
	])('reads the type of $label', ({ id, expected }) => {
		expect(nodeTypeOfId(id)).toBe(expected);
		expect(isNodeId(id)).toBe(expected !== null);
	});
});

describe('the endpoint a node reaches', () => {
	it.each([
		{ label: 'an Anthropic node', node: node({ type: 'anthropic-compatible' }), path: '/messages' },
		{
			label: 'an Anthropic node that also carries an api type',
			node: node({ type: 'anthropic-compatible', api_type: 'responses' }),
			path: '/messages'
		},
		{ label: 'an OpenAI chat node', node: node(), path: '/chat/completions' },
		{
			label: 'an OpenAI responses node',
			node: node({ api_type: 'responses' }),
			path: '/responses'
		},
		{
			label: 'an OpenAI node with no api type',
			node: node({ api_type: undefined }),
			path: '/chat/completions'
		}
	])('derives the path of $label', ({ node: subject, path }) => {
		expect(nodeEndpointPath(subject)).toBe(path);
	});

	it('names the endpoint with the API it reaches', () => {
		expect(nodeEndpointLabel(node({ type: 'anthropic-compatible' }))).toBe('Messages API');
		expect(nodeEndpointLabel(node())).toBe('Chat Completions');
		expect(nodeEndpointLabel(node({ api_type: 'responses' }))).toBe('Responses API');
	});

	it.each([
		{
			label: 'a base URL with no trailing slash',
			base: 'https://llm.example.com/v1',
			expected: 'https://llm.example.com/v1/chat/completions'
		},
		{
			label: 'a base URL with one trailing slash',
			base: 'https://llm.example.com/v1/',
			expected: 'https://llm.example.com/v1/chat/completions'
		},
		{
			label: 'a base URL with a run of trailing slashes',
			base: 'https://llm.example.com/v1//',
			expected: 'https://llm.example.com/v1/chat/completions'
		}
	])('joins $label with exactly one separator', ({ base, expected }) => {
		expect(nodeEndpointUrl(node({ base_url: base }))).toBe(expected);
	});
});

describe('the node wire shape', () => {
	it('parses a node, with the api type absent for an Anthropic node', () => {
		const parsed = schemaProviderNode.safeParse(
			node({ type: 'anthropic-compatible', api_type: undefined })
		);
		expect(parsed.success).toBe(true);
		expect(parsed.success && parsed.data.api_type).toBeUndefined();
	});

	it('parses the list envelope, which carries no meta block', () => {
		const parsed = schemaProviderNodeList.safeParse({
			data: [node(), node({ id: 'openai-compatible-02' })]
		});
		expect(parsed.success).toBe(true);
		expect(parsed.success && parsed.data.data).toHaveLength(2);
	});

	it('refuses a node with no prefix', () => {
		expect(schemaProviderNode.safeParse({ ...node(), prefix: '' }).success).toBe(false);
	});
});

describe('the probe state', () => {
	it.each([
		{ label: 'the ok state', state: 'ok', expected: 'Reachable' },
		{ label: 'the fail state', state: 'fail', expected: 'Failed' },
		{ label: 'a state the panel does not know', state: 'timeout', expected: 'timeout' }
	])('labels $label', ({ state, expected }) => {
		expect(nodeTestStateLabel(state)).toBe(expected);
	});
});
