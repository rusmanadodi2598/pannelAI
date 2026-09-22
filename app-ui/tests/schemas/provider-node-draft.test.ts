// Tests for the custom provider draft and the bodies a write carries (docs/SPEC-API/001-SPEC-API.md §7.4,
// docs/SPEC-UI/001-SPEC-UI.md §6.3).
//
// Two of the rules checked here are the gateway's own, and a save the panel lets through would be a save
// the gateway refuses: a prefix that can serve as a model-string namespace, and `api_type` belonging to an
// OpenAI-compatible node alone. The third is the panel's own and is about honesty at the wire: the base URL
// is what the gateway appends a path to, so the trailing slash is dropped here rather than producing a
// doubled separator the operator would meet as a transport error.
//
// The endpoint a stored node reaches and the list envelope are in `provider-node.test.ts`; this file never
// parses a response body.

import { describe, expect, it } from 'vitest';
import {
	NODE_BASE_URL_HINTS,
	createProviderNodeBody,
	customProviderDraftFrom,
	customProviderDraftNew,
	schemaCustomProviderDraft,
	updateProviderNodeBody,
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

const draft = (overrides: Record<string, unknown> = {}) => ({
	name: 'OpenAI Compatible (prod)',
	prefix: 'mycorp',
	api_type: 'chat',
	base_url: 'https://llm.example.com/v1',
	...overrides
});

describe('the node draft', () => {
	it('accepts a valid draft and returns the transformed values', () => {
		const parsed = schemaCustomProviderDraft.safeParse(
			draft({ base_url: 'https://llm.example.com/v1/' })
		);
		expect(parsed.success).toBe(true);
		expect(parsed.success && parsed.data.base_url).toBe('https://llm.example.com/v1');
	});

	it('trims the name', () => {
		const parsed = schemaCustomProviderDraft.safeParse(draft({ name: '  Prod node  ' }));
		expect(parsed.success && parsed.data.name).toBe('Prod node');
	});

	it.each([
		{ label: 'an empty prefix', prefix: '', message: 'A prefix is required.' },
		{ label: 'a prefix of spaces', prefix: '   ', message: 'A prefix is required.' },
		{
			label: 'a prefix with a slash',
			prefix: 'mycorp/prod',
			message: 'Use letters, digits, dots, dashes, and underscores only.'
		},
		{
			label: 'a prefix with a space',
			prefix: 'my corp',
			message: 'Use letters, digits, dots, dashes, and underscores only.'
		},
		{
			label: 'a prefix with a colon',
			prefix: 'mycorp:prod',
			message: 'Use letters, digits, dots, dashes, and underscores only.'
		},
		{
			label: 'a prefix past 64 characters',
			prefix: 'a'.repeat(65),
			message: 'Use 64 characters or fewer.'
		}
	])('refuses $label', ({ prefix, message }) => {
		const parsed = schemaCustomProviderDraft.safeParse(draft({ prefix }));
		expect(parsed.success).toBe(false);
		expect(!parsed.success && parsed.error.issues[0]?.message).toBe(message);
	});

	it.each(['mycorp', 'my-corp', 'my_corp', 'my.corp', 'MyCorp9'])(
		'accepts the prefix %s',
		(prefix) => {
			expect(schemaCustomProviderDraft.safeParse(draft({ prefix })).success).toBe(true);
		}
	);

	it.each([
		{ label: 'a base URL with no scheme', base_url: 'llm.example.com/v1' },
		{ label: 'a base URL with an unsupported scheme', base_url: 'ftp://llm.example.com/v1' },
		{ label: 'an empty base URL', base_url: '' },
		{ label: 'a base URL that cannot be parsed', base_url: 'https://' }
	])('refuses $label', ({ base_url }) => {
		expect(schemaCustomProviderDraft.safeParse(draft({ base_url })).success).toBe(false);
	});

	it('refuses an api type the gateway does not accept', () => {
		expect(schemaCustomProviderDraft.safeParse(draft({ api_type: 'completions' })).success).toBe(
			false
		);
	});

	it('refuses an extra field, so a body cannot carry a rule nobody checked', () => {
		expect(
			schemaCustomProviderDraft.safeParse({ ...draft(), type: 'openai-compatible' }).success
		).toBe(false);
	});

	it('refuses a name with angle brackets, which the shared label primitive rejects', () => {
		expect(schemaCustomProviderDraft.safeParse(draft({ name: '<script>' })).success).toBe(false);
	});
});

describe('drafts', () => {
	it('starts a new draft blank, with chat as the api type', () => {
		expect(customProviderDraftNew()).toEqual({
			name: '',
			prefix: '',
			api_type: 'chat',
			base_url: ''
		});
	});

	it('seeds an edit from the stored node', () => {
		expect(customProviderDraftFrom(node())).toEqual({
			name: 'OpenAI Compatible (prod)',
			prefix: 'mycorp',
			api_type: 'chat',
			base_url: 'https://llm.example.com/v1'
		});
	});

	it('seeds an edit of a responses node with the responses api type', () => {
		expect(customProviderDraftFrom(node({ api_type: 'responses' })).api_type).toBe('responses');
	});

	it('seeds an unknown api type as chat rather than as an invalid select value', () => {
		expect(customProviderDraftFrom(node({ api_type: 'legacy' })).api_type).toBe('chat');
	});

	it('names the base URL each type is usually pointed at', () => {
		expect(NODE_BASE_URL_HINTS['openai-compatible']).toBe('https://api.openai.com/v1');
		expect(NODE_BASE_URL_HINTS['anthropic-compatible']).toBe('https://api.anthropic.com/v1');
	});
});

describe('request bodies', () => {
	it('carries the api type on an OpenAI-compatible create', () => {
		const parsed = schemaCustomProviderDraft.parse(draft({ api_type: 'responses' }));
		expect(createProviderNodeBody('openai-compatible', parsed)).toEqual({
			name: 'OpenAI Compatible (prod)',
			prefix: 'mycorp',
			type: 'openai-compatible',
			api_type: 'responses',
			base_url: 'https://llm.example.com/v1'
		});
	});

	it('omits the api type on an Anthropic-compatible create, which the gateway refuses it on', () => {
		const body = createProviderNodeBody(
			'anthropic-compatible',
			schemaCustomProviderDraft.parse(draft())
		);
		expect(body).toEqual({
			name: 'OpenAI Compatible (prod)',
			prefix: 'mycorp',
			type: 'anthropic-compatible',
			base_url: 'https://llm.example.com/v1'
		});
		expect('api_type' in body).toBe(false);
	});

	it('patches only the three fields the route accepts', () => {
		const body = updateProviderNodeBody(schemaCustomProviderDraft.parse(draft()));
		expect(body).toEqual({
			name: 'OpenAI Compatible (prod)',
			prefix: 'mycorp',
			base_url: 'https://llm.example.com/v1'
		});
		expect('type' in body).toBe(false);
		expect('api_type' in body).toBe(false);
	});
});
