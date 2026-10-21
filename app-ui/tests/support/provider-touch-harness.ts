// The fixtures and render helpers the two provider touch-target files share
// (docs/SPEC-UI/001-SPEC-UI.md §8.7, §11.3).
//
// F1 holds the copy control in both model tables and F3 holds every retry and clear text button on the
// provider screens; both need the catalog row, the catalog render, the search that narrows it, and the one
// class contract they assert. They live here so the two files cannot drift apart on what "touch sized" means.

import { fireEvent, render, within } from '@testing-library/svelte';
import { expect, vi } from 'vitest';
import ModelCatalogList from '../../src/lib/components/ModelCatalogList.svelte';
import { createModelDisabledStore } from '../../src/lib/stores/model-disabled.svelte';
import { createModelTestStore } from '../../src/lib/stores/model-test.svelte';
import { createProviderThinkingStore } from '../../src/lib/stores/provider-thinking.svelte';

export const NODE_ID = 'openai-compatible-01J';
export const NODE_PREFIX = 'mycorp';

export function catalogRow(): Record<string, unknown> {
	return {
		id: 'openai/gpt-4o',
		provider_id: 'openai',
		model_id: 'gpt-4o',
		display_name: 'GPT-4o',
		kind: 'chat',
		capabilities: ['vision', 'tools'],
		source: 'registry'
	};
}

/** Serves the model catalog read the way `model-catalog.test.ts` does. */
export function stubCatalog(data: Record<string, unknown>[], status = 200): void {
	vi.stubGlobal(
		'fetch',
		async () =>
			new Response(JSON.stringify({ data }), {
				status,
				headers: { 'content-type': 'application/json' }
			})
	);
}

/** The catalog list, rendered from a read the caller already stubbed, for a case that narrows it first. */
export function renderStubbedCatalog(): HTMLElement {
	const { container } = render(ModelCatalogList, {
		props: {
			providerId: 'openai',
			disabled: createModelDisabledStore(),
			thinking: createProviderThinkingStore(),
			tests: createModelTestStore(),
			onchanged: () => {}
		}
	});

	return container;
}

export function renderCatalogList(data: Record<string, unknown>[], status = 200): void {
	stubCatalog(data, status);
	renderStubbedCatalog();
}

/** Applies the search box the way a keyboard user would. */
export async function search(container: HTMLElement, term: string): Promise<void> {
	await fireEvent.input(within(container).getByLabelText('Search models'), {
		target: { value: term }
	});
	await fireEvent.submit(container.querySelector('form') as HTMLFormElement);
}

/** The 44px text-button contract every retry, clear and text link on these screens meets (§8.7). */
export function expectTouchTextButton(element: HTMLElement, name: string): void {
	const classes = element.className.split(/\s+/);
	for (const expected of ['inline-flex', 'min-h-11', 'items-center', 'underline']) {
		expect(classes, `${name} is missing ${expected}`).toContain(expected);
	}
}
