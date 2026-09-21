// Skill source derivation tests (docs/SPEC-UI/001-SPEC-UI.md §6.10).
//
// Three pure decisions the screen depends on: the order the rows render in, the line the copy control
// hands out, and how a source request's answer reads. The last one is the honesty rule of §6.10, so each
// cause is asserted separately rather than as one "it failed" case.

import { describe, expect, it } from 'vitest';
import {
	SOURCE_TIMEOUT_MS,
	classifySourceFailure,
	classifySourceStatus,
	installLine,
	orderSkills
} from '$lib/schemas/skill-source';
import type { Skill } from '$lib/schemas/skill';
import { DEFAULT_ROWS, skillRow } from '../support/skill-catalog';

const SKILLS = DEFAULT_ROWS as unknown as Skill[];

describe('installLine', () => {
	it('composes the line from the row own address rather than storing it', () => {
		expect(installLine(SKILLS[0])).toBe(
			'Read this skill and use it: https://raw.githubusercontent.com/rusmanadodi2598/pannelAI/refs/heads/main/skills/pannelai/SKILL.md'
		);
	});
});

describe('orderSkills', () => {
	it('leaves an entry-first catalog in the order it arrived', () => {
		expect(orderSkills(SKILLS).map((skill) => skill.id)).toEqual([
			'pannelai',
			'pannelai-chat',
			'pannelai-image'
		]);
	});

	it('lifts the entry skill to the front of a catalog that sends it last', () => {
		const last = [
			skillRow('pannelai-chat', 'Chat', '/chat/completions') as unknown as Skill,
			skillRow('pannelai-image', 'Image Generation', '/images/generations') as unknown as Skill,
			skillRow('pannelai', 'pannelAI (Entry)', null, true) as unknown as Skill
		];

		expect(orderSkills(last).map((skill) => skill.id)).toEqual([
			'pannelai',
			'pannelai-chat',
			'pannelai-image'
		]);
	});

	it('keeps every row when the catalog carries no entry skill', () => {
		const none = SKILLS.filter((skill) => !skill.entry);

		expect(orderSkills(none).map((skill) => skill.id)).toEqual(['pannelai-chat', 'pannelai-image']);
	});
});

describe('classifySourceStatus', () => {
	it('reads a 2xx as the document being there', () => {
		expect(classifySourceStatus(200)).toEqual({ state: 'available' });
		expect(classifySourceStatus(204)).toEqual({ state: 'available' });
	});

	it('reads 404 and 410 as the file not being published', () => {
		expect(classifySourceStatus(404)).toEqual({
			state: 'unavailable',
			cause: { kind: 'missing', status: 404 }
		});
		expect(classifySourceStatus(410)).toEqual({
			state: 'unavailable',
			cause: { kind: 'missing', status: 410 }
		});
	});

	it('reports any other status with its number rather than guessing a cause', () => {
		expect(classifySourceStatus(500)).toEqual({
			state: 'unavailable',
			cause: { kind: 'http', status: 500 }
		});
		expect(classifySourceStatus(403)).toEqual({
			state: 'unavailable',
			cause: { kind: 'http', status: 403 }
		});
	});
});

describe('classifySourceFailure', () => {
	it('reads an aborted request as the timeout, in seconds', () => {
		const timeout = new Error('signal timed out');
		timeout.name = 'TimeoutError';

		expect(classifySourceFailure(timeout, 8000)).toEqual({
			state: 'unavailable',
			cause: { kind: 'timeout', seconds: 8 }
		});
	});

	it('reads a cancelled request the same way, because the panel cancels nothing else', () => {
		const aborted = new Error('aborted');
		aborted.name = 'AbortError';

		expect(classifySourceFailure(aborted)).toEqual({
			state: 'unavailable',
			cause: { kind: 'timeout', seconds: SOURCE_TIMEOUT_MS / 1000 }
		});
	});

	it('carries the detail of a request that never completed', () => {
		expect(classifySourceFailure(new Error('Failed to fetch'))).toEqual({
			state: 'unavailable',
			cause: { kind: 'network', detail: 'Failed to fetch' }
		});
	});

	it('reports a thrown non-Error rather than calling it a failure with no reason', () => {
		expect(classifySourceFailure('offline')).toEqual({
			state: 'unavailable',
			cause: { kind: 'network', detail: 'offline' }
		});
	});
});
