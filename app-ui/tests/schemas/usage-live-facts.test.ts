// Live fact derivation tests (src/lib/schemas/usage-live-view.ts).
//
// Split from `usage-live-view.test.ts`, which the facts rows would have pushed past the line limit (§1.1).
// The facts are the three live states the row may state in words, and they live in the row rather than in
// the drawing's frame: the frame is hidden from assistive technology, so this derivation is what decides
// what that tab may say. §2.5: both functions run a table, and each row is
// reported on its own name.

import { describe, expect, it } from 'vitest';
import type { UsageLiveActive } from '$lib/schemas/usage-live';
import {
	activeComboNames,
	directCount,
	liveFacts,
	providerDisplayName,
	type LiveFact
} from '$lib/schemas/usage-live-view';
import { forEachCase } from '../support/tables';

const PROVIDERS = [
	{ id: 'openai', name: 'OpenAI' },
	{ id: 'anthropic', name: 'Anthropic' }
];

function entry(providerId: string, model?: string, combo?: string): UsageLiveActive {
	return {
		provider_id: providerId,
		started_at: '2026-09-27T10:00:00Z',
		...(model === undefined ? {} : { model }),
		...(combo === undefined ? {} : { combo })
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
		// Absence is stated by the other tabs (the connection chip names the idle state), never by a
		// sentence that never changes.
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
		// The drawing encodes the combo, so the row has to state it: the drawing is hidden from assistive
		// technology and this row is what says in words what its nodes claim (SPEC-UI §6.5).
		name: 'names the combo a request entered through before the upstream that answered',
		active: [entry('openai', 'gpt-4o', 'pro-tier')],
		last: '',
		error: '',
		expected: [{ label: '1 in flight', value: 'pro-tier → OpenAI (gpt-4o)', tone: 'status' }]
	},
	{
		name: 'states no arrow for a frame that carries no combo',
		active: [entry('openai', 'gpt-4o', '   ')],
		last: '',
		error: '',
		expected: [{ label: '1 in flight', value: 'OpenAI (gpt-4o)', tone: 'status' }]
	},
	{
		// A combo is addressed by a name the operator typed, and `openai` is a legal one. Resolving it
		// through the provider registry would restate a combo as a vendor and light the wrong fact.
		name: 'leaves a combo whose name matches a provider as its own name',
		active: [entry('openai', 'gpt-4o', 'openai')],
		last: '',
		error: '',
		expected: [{ label: '1 in flight', value: 'openai → OpenAI (gpt-4o)', tone: 'status' }]
	},
	{
		name: 'names the combo on every entry that carries one',
		active: [entry('openai', 'gpt-4o', 'pro-tier'), entry('anthropic', undefined, 'fast')],
		last: '',
		error: '',
		expected: [
			{
				label: '2 in flight',
				value: 'pro-tier → OpenAI (gpt-4o), fast → Anthropic',
				tone: 'status'
			}
		]
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

describe('directCount', () => {
	const directCases = [
		{
			name: 'nothing in flight',
			active: [] as UsageLiveActive[],
			expected: 0
		},
		{
			name: 'every entry addressed a single model',
			active: [entry('openai'), entry('anthropic', 'claude')],
			expected: 2
		},
		{
			name: 'every entry entered through a combo',
			active: [entry('openai', 'gpt-4o', 'pro-tier'), entry('anthropic', undefined, 'fast')],
			expected: 0
		},
		{
			// The drawing lights its direct hop from this figure, so a blank on the wire counts as no combo
			// rather than as a combo with an empty name.
			name: 'a combo field that is only whitespace is no combo',
			active: [entry('openai', 'gpt-4o', '   ')],
			expected: 1
		}
	];

	forEachCase(directCases, (testCase) => {
		expect(directCount(testCase.active)).toBe(testCase.expected);
	});
});

describe('activeComboNames', () => {
	const nameCases = [
		{
			name: 'nothing in flight',
			active: [] as UsageLiveActive[],
			expected: [] as string[]
		},
		{
			name: 'entries that carry no combo',
			active: [entry('openai'), entry('anthropic')],
			expected: []
		},
		{
			name: 'one combo behind two upstreams, as a fusion combo fails over',
			active: [entry('openai', 'gpt-4o', 'pro-tier'), entry('anthropic', 'claude', 'pro-tier')],
			expected: ['pro-tier']
		},
		{
			name: 'two combos in flight, in the order the frame reports them',
			active: [entry('openai', 'gpt-4o', 'fast'), entry('anthropic', 'claude', 'pro-tier')],
			expected: ['fast', 'pro-tier']
		},
		{
			name: 'a combo whose name is only whitespace',
			active: [entry('openai', 'gpt-4o', '   ')],
			expected: []
		}
	];

	forEachCase(nameCases, (testCase) => {
		expect(activeComboNames(testCase.active)).toEqual(testCase.expected);
	});

	it('keeps the spelling the gateway sent, because the band compares it exactly', () => {
		expect(activeComboNames([entry('openai', undefined, 'Pro-Tier')])).toEqual(['Pro-Tier']);
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
