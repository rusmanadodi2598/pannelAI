// The pasted-list planner behind the provider detail screen's Add API Key dialog
// (docs/SPEC-UI/001-SPEC-UI.md §6.3, SPEC-API §7.5).
//
// What it has to get right is that a name is an identity here: (provider_id, label) is unique in app-serv
// (`app-serv/migrations/000005_upstream_endpoints.up.sql:29`), so a generated name that collides refuses
// the whole paste. The cases below pin both halves of that: the name a line ends up with, and the line
// number a refusal is keyed to.

import { describe, expect, it } from 'vitest';
import { planConnectionLines } from '$lib/schemas/connection-plan';

describe('planConnectionLines', () => {
	it('keeps the name a line was given', () => {
		expect(planConnectionLines('production|sk-abcdefgh')).toEqual([
			{ line: 1, label: 'production', value: 'sk-abcdefgh' }
		]);
	});

	it('names a bare line rather than storing it without a name', () => {
		// A label is required on the wire (`app-serv/internal/schema/endpoint.go:91`), so a nameless line is
		// not an option. The reference names it by index and this is the same base (`bulkAdd.js:60`).
		expect(planConnectionLines('sk-abcdefgh')).toEqual([
			{ line: 1, label: 'Key 1', value: 'sk-abcdefgh' }
		]);
	});

	it('treats an empty name as no name at all', () => {
		expect(planConnectionLines('|sk-abcdefgh')).toEqual([
			{ line: 1, label: 'Key 1', value: 'sk-abcdefgh' }
		]);
	});

	it('numbers bare lines in the order they were pasted', () => {
		expect(planConnectionLines('sk-abcdefgh\nsk-ijklmnop').map((row) => row.label)).toEqual([
			'Key 1',
			'Key 2'
		]);
	});

	it('keeps the source line number, so a blank line does not shift the rows after it', () => {
		expect(planConnectionLines('production|sk-abcdefgh\n\nbackup|sk-ijklmnop')).toEqual([
			{ line: 1, label: 'production', value: 'sk-abcdefgh' },
			{ line: 3, label: 'backup', value: 'sk-ijklmnop' }
		]);
	});

	it('splits on the first bar only, so a key containing one survives', () => {
		expect(planConnectionLines('production|sk-ab|cd')).toEqual([
			{ line: 1, label: 'production', value: 'sk-ab|cd' }
		]);
	});

	it('gap-fills around the names already stored, so a second paste cannot collide', () => {
		expect(planConnectionLines('sk-abcdefgh', ['Key 1'])).toEqual([
			{ line: 1, label: 'Key 2', value: 'sk-abcdefgh' }
		]);
	});

	it('gap-fills around the names it assigned earlier in the same paste', () => {
		expect(planConnectionLines('sk-abcdefgh\nsk-ijklmnop', ['Key 1', 'Key 3'])).toEqual([
			{ line: 1, label: 'Key 2', value: 'sk-abcdefgh' },
			{ line: 2, label: 'Key 4', value: 'sk-ijklmnop' }
		]);
	});

	it('takes the next free suffix for a typed name that is already stored', () => {
		expect(planConnectionLines('production|sk-abcdefgh', ['production'])).toEqual([
			{ line: 1, label: 'production 1', value: 'sk-abcdefgh' }
		]);
	});

	it('avoids a stored name that differs only in case, as the reference planner does', () => {
		// The index compares the stored bytes, so `Key 1` and `KEY 1` are two rows to it. The planner stays
		// case-blind anyway, following the reference (`bulkAdd.js:80`): a name it refuses to reuse costs
		// nothing, while a collision costs the whole paste.
		expect(planConnectionLines('sk-abcdefgh', ['KEY 1'])).toEqual([
			{ line: 1, label: 'Key 2', value: 'sk-abcdefgh' }
		]);
	});
});
