// Usage live derivation tests (src/lib/schemas/usage-live-view.ts, draft 012 F2).
//
// Three subjects, one describe each: the fold that keeps the stream away from the aggregates, the guard
// that ages out a marker the gateway never cleared, and the label rule that stops the panel calling a dead
// socket Live. The drawing's own derivations are in `usage-topology-view.test.ts`.

import { describe, expect, it } from 'vitest';
import type { UsageLiveActive } from '$lib/schemas/usage-live';
import {
	FE_ACTIVE_TIMEOUT_MS,
	freshActive,
	liveMerge,
	streamLabel,
	streamStatus,
	type LiveView,
	type StreamInput,
	type StreamStatus
} from '$lib/schemas/usage-live-view';
import { forEachCase } from '../support/tables';

const NOW = Date.parse('2026-09-22T10:00:00Z');

function at(offsetMs: number): string {
	return new Date(NOW + offsetMs).toISOString();
}

function activeEntry(providerId: string, offsetMs = 0): UsageLiveActive {
	return { provider_id: providerId, started_at: at(offsetMs) };
}

describe('liveMerge', () => {
	it('takes the three live fields from the frame and stamps when it landed', () => {
		const merged = liveMerge(
			null,
			{
				active: [activeEntry('openai')],
				recent: [{ request_id: 'req_1', provider_id: 'anthropic' }],
				error_provider: 'openai'
			},
			NOW
		);

		expect(merged.active).toEqual([activeEntry('openai')]);
		expect(merged.recent[0].request_id).toBe('req_1');
		expect(merged.errorProvider).toBe('openai');
		expect(merged.receivedAt).toBe(NOW);
	});

	it('holds no field an aggregate could be written into', () => {
		// The proof is the shape, not a value: a frame carrying totals has nowhere in this type to put them,
		// so the stream cannot overwrite what the REST reads returned (draft 012 F2).
		const merged = liveMerge(null, { active: [], recent: [], error_provider: '' }, NOW);

		expect(Object.keys(merged).sort()).toEqual(['active', 'errorProvider', 'receivedAt', 'recent']);
	});

	it('starts empty when there is no previous state', () => {
		const merged = liveMerge(null, {}, NOW);

		expect(merged.active).toEqual([]);
		expect(merged.recent).toEqual([]);
		expect(merged.errorProvider).toBe('');
	});

	const holdCases = [
		{
			name: 'active',
			previous: { active: [activeEntry('openai')], recent: [], errorProvider: '', receivedAt: NOW },
			frame: { recent: [{ request_id: 'req_2', provider_id: 'anthropic' }] },
			check: (merged: LiveView) => expect(merged.active).toEqual([activeEntry('openai')])
		},
		{
			name: 'recent',
			previous: {
				active: [],
				recent: [{ request_id: 'req_1', provider_id: 'openai' }],
				errorProvider: '',
				receivedAt: NOW
			},
			frame: { active: [] },
			check: (merged: LiveView) => expect(merged.recent[0].request_id).toBe('req_1')
		},
		{
			name: 'error provider',
			previous: { active: [], recent: [], errorProvider: 'openai', receivedAt: NOW },
			frame: { active: [] },
			check: (merged: LiveView) => expect(merged.errorProvider).toBe('openai')
		}
	];

	forEachCase(holdCases, (testCase) => {
		const merged = liveMerge(testCase.previous, testCase.frame, NOW + 1_000);

		testCase.check(merged);
		expect(merged.receivedAt).toBe(NOW + 1_000);
	});

	it('clears a field the frame states as empty', () => {
		const previous: LiveView = {
			active: [activeEntry('openai')],
			recent: [{ request_id: 'req_1', provider_id: 'openai' }],
			errorProvider: 'openai',
			receivedAt: NOW
		};

		const merged = liveMerge(previous, { active: [], recent: [], error_provider: '' }, NOW + 1_000);

		expect(merged.active).toEqual([]);
		expect(merged.recent).toEqual([]);
		expect(merged.errorProvider).toBe('');
	});
});

describe('freshActive', () => {
	const cases = [
		{ name: 'a request that just started', offsetMs: -1_000, kept: true },
		{ name: 'a request one second short of the guard', offsetMs: -59_999, kept: true },
		{ name: 'a request exactly at the guard', offsetMs: -FE_ACTIVE_TIMEOUT_MS, kept: false },
		{ name: 'a request past the guard', offsetMs: -FE_ACTIVE_TIMEOUT_MS - 1, kept: false },
		{ name: 'a request whose start is in the future', offsetMs: 5_000, kept: true }
	];

	forEachCase(cases, (testCase) => {
		const entries = freshActive([activeEntry('openai', testCase.offsetMs)], NOW);

		expect(entries).toHaveLength(testCase.kept ? 1 : 0);
	});

	it('drops only the stale entries and keeps the frame order', () => {
		const entries = freshActive(
			[
				activeEntry('stale', -FE_ACTIVE_TIMEOUT_MS - 1),
				activeEntry('openai', -2_000),
				activeEntry('anthropic', -1_000)
			],
			NOW
		);

		expect(entries.map((entry) => entry.provider_id)).toEqual(['openai', 'anthropic']);
	});

	it('takes the window as a parameter, so the guard is one figure in one place', () => {
		expect(freshActive([activeEntry('openai', -5_000)], NOW, 1_000)).toEqual([]);
		expect(freshActive([activeEntry('openai', -5_000)], NOW, 10_000)).toHaveLength(1);
	});
});

describe('streamStatus', () => {
	const base: StreamInput = { paused: false, fresh: false, down: false, active: false };

	const cases: { name: string; input: StreamInput; status: StreamStatus }[] = [
		{
			name: 'a connection that has not opened',
			input: { ...base, active: true },
			status: 'connecting'
		},
		{
			name: 'a connection delivering frames',
			input: { ...base, active: true, fresh: true },
			status: 'live'
		},
		{
			name: 'a connection that failed',
			input: { ...base, active: true, fresh: true, down: true },
			status: 'unavailable'
		},
		{
			name: 'a failure while nothing is being attempted',
			input: { ...base, down: true },
			status: 'unavailable'
		},
		{
			name: 'a pause over a live connection',
			input: { ...base, active: true, fresh: true, paused: true },
			status: 'paused'
		},
		{
			name: 'a pause over a failed connection',
			input: { ...base, down: true, paused: true },
			status: 'paused'
		},
		{ name: 'a stream that was never started', input: base, status: 'idle' },
		{
			name: 'frames on a connection that is no longer open',
			input: { ...base, fresh: true },
			status: 'idle'
		}
	];

	forEachCase(cases, (testCase) => {
		expect(streamStatus(testCase.input)).toBe(testCase.status);
	});
});

describe('streamLabel', () => {
	const statuses: StreamStatus[] = ['idle', 'connecting', 'live', 'paused', 'unavailable'];

	forEachCase(
		statuses.map((status) => ({ name: status, status })),
		(testCase) => {
			// R-36: the word Live is a claim that frames are arriving, so exactly one status may carry it.
			expect(streamLabel(testCase.status) === 'Live').toBe(testCase.status === 'live');
		}
	);
});
