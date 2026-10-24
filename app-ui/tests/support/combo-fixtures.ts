// The combo row and form fixtures (docs/SPEC-API/001-SPEC-API.md §7.6, docs/SPEC-UI/001-SPEC-UI.md §6.4).
//
// Four files read the combo contract: the row schemas, the order operations, the form, and the body a save
// writes. These two shapes are what all four build on, so they live once.

import type { ComboForm } from '$lib/schemas/combo-form';

export function combo(overrides: Record<string, unknown> = {}): Record<string, unknown> {
	return {
		id: 'cmb_1',
		name: 'daily',
		strategy: 'fallback',
		sticky_limit: 1,
		judge_model: '',
		models: [{ ref: 'openai/gpt-4o', priority: 0 }],
		created_at: '2026-09-18T00:00:00Z',
		updated_at: '2026-09-18T00:00:00Z',
		...overrides
	};
}

export function form(overrides: Partial<ComboForm> = {}): ComboForm {
	return {
		name: 'daily',
		strategy: 'fallback',
		models: [{ ref: 'openai/gpt-4o', priority: 0 }],
		stickyLimit: 1,
		judgeModel: '',
		...overrides
	};
}
