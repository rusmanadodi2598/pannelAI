// Live fact derivation tests (src/lib/schemas/usage-live-view.ts, draft 035 F2).
//
// Split from `usage-live-view.test.ts`, which the facts rows would have pushed past the line limit,
// the same way draft 023 split the icon tests on their seam (§1.1). The facts are the three live states
// the row may state in words, and they left the drawing's frame on purpose: the owner's correction of
// 2026-09-27 moved the sentence out of the node animation and into a tab of the live row, so this
// derivation is what decides what that tab may say. §2.5: both functions run a table, and each row is
// reported on its own name.

import { describe, expect, it } from 'vitest';
import type { UsageLiveActive } from '$lib/schemas/usage-live';
import { liveFacts, providerDisplayName, type LiveFact } from '$lib/schemas/usage-live-view';
import { forEachCase } from '../support/tables';

const PROVIDERS = [
	{ id: 'openai', name: 'OpenAI' },
	{ id: 'anthropic', name: 'Anthropic' }
];

function entry(providerId: string, model?: string): UsageLiveActive {
	return {
		provider_id: providerId,
		started_at: '2026-09-27T10:00:00Z',
		...(model === undefined ? {} : { model })
	};
}

type FactsCase = {
	name: string;
	active: UsageLiveActive[];
	last: string;
	error: string;
	expected: LiveFact[];
};

const FACTS_CASES: FactsCase[] = [
	{
		// The owner's correction of 2026-09-23 survives the move: absence is stated by the other tabs
		// (the connection chip names the idle state), never by a sentence that never changes.
		name: 'states no fact while the stream has reported nothing that happened',
		active: [],
		last: '',
		error: '',
		expected: []
	},
	{
		name: 'names what is in flight with the model the frame reported, in the status tone',
		active: [entry('openai', 'gpt-4o'), entry('anthropic')],
		last: '',
		error: '',
		expected: [{ label: '2 in flight', value: 'OpenAI (gpt-4o), Anthropic', tone: 'status' }]
	},
	{
		// The wire's model is optional, and the type carries no minimum: a blank on the wire is no name.
		name: 'draws no empty parenthesis for a frame whose model is blank',
		active: [entry('openai', '   ')],
		last: '',
		error: '',
		expected: [{ label: '1 in flight', value: 'OpenAI', tone: 'status' }]
	},
	{
		name: 'states the finished and error facts plainly, in the order the stream reports them',
		active: [],
		last: 'anthropic',
		error: 'openai',
		expected: [
			{ label: 'Last finished', value: 'Anthropic', tone: 'plain' },
			{ label: 'Last error', value: 'OpenAI', tone: 'plain' }
		]
	},
	{
		name: 'keeps in flight first when all three facts are on the wire',
		active: [entry('openai')],
		last: 'anthropic',
		error: 'anthropic',
		expected: [
			{ label: '1 in flight', value: 'OpenAI', tone: 'status' },
			{ label: 'Last finished', value: 'Anthropic', tone: 'plain' },
			{ label: 'Last error', value: 'Anthropic', tone: 'plain' }
		]
	},
	{
		name: 'names a provider the registry does not carry by its id',
		active: [],
		last: 'gone-provider',
		error: '',
		expected: [{ label: 'Last finished', value: 'gone-provider', tone: 'plain' }]
	}
];

describe('liveFacts', () => {
	forEachCase(FACTS_CASES, (testCase) => {
		expect(liveFacts(PROVIDERS, testCase.active, testCase.last, testCase.error)).toEqual(
			testCase.expected
		);
	});

	it('ends no value with a period, because the tab is a label and not a paragraph', () => {
		for (const fact of liveFacts(PROVIDERS, [entry('openai')], 'anthropic', 'openai')) {
			expect(fact.value.endsWith('.')).toBe(false);
			expect(fact.label.endsWith('.')).toBe(false);
		}
	});
});

type NameCase = { name: string; id: string; expected: string };

const NAME_CASES: NameCase[] = [
	{
		name: 'resolves a registry name whatever case the frame reported the id in',
		id: 'OPENAI',
		expected: 'OpenAI'
	},
	{ name: 'resolves the exact id too', id: 'anthropic', expected: 'Anthropic' },
	{
		name: 'falls back to the id when the registry carries no entry',
		id: 'gone-provider',
		expected: 'gone-provider'
	}
];

describe('providerDisplayName', () => {
	forEachCase(NAME_CASES, (testCase) => {
		expect(providerDisplayName(PROVIDERS, testCase.id)).toBe(testCase.expected);
	});
});
