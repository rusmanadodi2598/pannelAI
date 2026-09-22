// Usage live frame tests (src/lib/schemas/usage-live.ts, draft 010 §10.3).
//
// The frame is a contract the gateway does not serve yet, so these rows are the panel's statement of what
// it will accept. Two rules run through them: a collection spelled `null` is the same as one spelled `[]`,
// and a field the screen does not draw cannot fail the frame.

import { describe, expect, it } from 'vitest';
import { schemaUsageLiveFrame } from '$lib/schemas/usage-live';
import { forEachCase } from '../support/tables';

const STARTED = '2026-09-22T10:00:00Z';
const RECENT_TS = '2026-09-22T09:59:00Z';

function frame(overrides: Record<string, unknown> = {}): Record<string, unknown> {
	return {
		active: [{ provider_id: 'openai', model: 'gpt-4o', started_at: STARTED }],
		recent: [{ request_id: 'req_1', provider_id: 'anthropic', ts: RECENT_TS, status: 'success' }],
		error_provider: '',
		...overrides
	};
}

describe('schemaUsageLiveFrame', () => {
	it('reads a full frame', () => {
		const parsed = schemaUsageLiveFrame.parse(frame());

		expect(parsed.active).toEqual([
			{ provider_id: 'openai', model: 'gpt-4o', started_at: STARTED }
		]);
		expect(parsed.recent?.[0].request_id).toBe('req_1');
		expect(parsed.error_provider).toBe('');
	});

	const nullCases = [
		{ name: 'null', value: null },
		{ name: 'an empty list', value: [] }
	];

	forEachCase(nullCases, (testCase) => {
		const parsed = schemaUsageLiveFrame.parse(frame({ active: testCase.value }));

		expect(parsed.active).toEqual([]);
	});

	it('leaves a collection the frame omits absent, so a merge can keep the last one', () => {
		const parsed = schemaUsageLiveFrame.parse({});

		expect(parsed.active).toBeUndefined();
		expect(parsed.recent).toBeUndefined();
		expect(parsed.error_provider).toBeUndefined();
	});

	it('names the provider that errored when the frame carries one', () => {
		expect(schemaUsageLiveFrame.parse(frame({ error_provider: 'openai' })).error_provider).toBe(
			'openai'
		);
	});

	it('strips a field it does not model rather than failing the frame', () => {
		const parsed = schemaUsageLiveFrame.parse(frame({ pending: 4, active: [] }));

		expect(parsed).not.toHaveProperty('pending');
		expect(
			schemaUsageLiveFrame.parse(
				frame({ active: [{ provider_id: 'openai', started_at: STARTED, account: 'a1' }] })
			).active?.[0]
		).not.toHaveProperty('account');
	});

	const rejections = [
		{
			name: 'an in-flight entry with no start instant',
			value: frame({ active: [{ provider_id: 'openai' }] })
		},
		{
			name: 'an in-flight entry with an unparseable start instant',
			value: frame({ active: [{ provider_id: 'openai', started_at: 'half past' }] })
		},
		{
			name: 'an in-flight entry with no provider',
			value: frame({ active: [{ provider_id: '', started_at: STARTED }] })
		},
		{
			name: 'a recent entry with no request id',
			value: frame({ recent: [{ provider_id: 'openai' }] })
		},
		{
			name: 'a recent entry with a status outside the closed set',
			value: frame({ recent: [{ request_id: 'req_1', provider_id: 'openai', status: 'timeout' }] })
		}
	];

	forEachCase(rejections, (testCase) => {
		expect(schemaUsageLiveFrame.safeParse(testCase.value).success).toBe(false);
	});
});
