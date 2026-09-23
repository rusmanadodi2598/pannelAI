// The display name rule for a custom model: optional on the form, filled from the model id on the wire
// (docs/SPEC-UI/001-SPEC-UI.md §6.3, draft 019 D2).
//
// Split from `tests/schemas/custom-model.test.ts` because the rule is its own concern and the pass's cases
// pushed that file past the panel's line warning: the reference adds a model from its id alone
// (`CompatibleModelsSection.js:106-118`), while the wire refuses an empty name
// (`app-serv/internal/schema/model.go:47`), so the two cases here are the seam between those two facts.

import { describe, expect, it } from 'vitest';
import { customModelBody, schemaCustomModelForm } from '$lib/schemas/custom-model';

const valid = { model_id: 'gpt-4o-mini', display_name: 'GPT-4o mini', capabilities: 'vision' };

describe('a custom model form with no display name', () => {
	it.each([
		{ label: 'an empty display name', display_name: '' },
		{ label: 'a display name that is only spaces', display_name: '   ' }
	])('accepts $label, because the model id is what the row shows then', ({ display_name }) => {
		expect(schemaCustomModelForm.safeParse({ ...valid, display_name }).success).toBe(true);
	});

	// A name that *is* typed is still checked: the bounds and the bracket rule are the wire's.
	it.each([
		{ label: 'past the API bound', display_name: 'a'.repeat(121) },
		{ label: 'with angle brackets', display_name: '<b>x</b>' }
	])('still refuses a name $label', ({ display_name }) => {
		expect(schemaCustomModelForm.safeParse({ ...valid, display_name }).success).toBe(false);
	});
});

describe('the body a blank display name becomes', () => {
	it.each([
		{ label: 'an empty field', display_name: '' },
		{ label: 'a field holding only spaces', display_name: '   ' }
	])('carries the model id when the field held $label', ({ display_name }) => {
		const body = customModelBody('openai', {
			model_id: 'gpt-4o-mini',
			display_name,
			capabilities: ''
		});

		expect(body.display_name).toBe('gpt-4o-mini');
	});

	it('keeps a typed name as it was typed', () => {
		const body = customModelBody('openai', {
			model_id: 'gpt-4o-mini',
			display_name: 'GPT-4o mini',
			capabilities: ''
		});

		expect(body.display_name).toBe('GPT-4o mini');
	});
});
