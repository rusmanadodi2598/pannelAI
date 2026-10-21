// The catalog fixture and render helpers the two probe test files share
// (docs/SPEC-UI/001-SPEC-UI.md §6.3).
//
// The page owns the probe store, so the test owns it too: one stub and one store per case, because the
// sweep's answers and the rows' states must be one record for the assertions to mean anything. The row
// lookup and the Test button are here for the same reason: both files reach a row the same way.

import { render, waitFor, within } from '@testing-library/svelte';
import { expect } from 'vitest';
import ModelCatalogList from '../../src/lib/components/ModelCatalogList.svelte';
import { createModelDisabledStore } from '$lib/stores/model-disabled.svelte';
import { createModelTestStore } from '$lib/stores/model-test.svelte';
import { createProviderThinkingStore } from '$lib/stores/provider-thinking.svelte';
import { catalogRow, stubModels, type ModelStub } from './model-stub';

export const chatRow = catalogRow();
export const imageRow = catalogRow({
	id: 'openai/dall-e-3',
	model_id: 'dall-e-3',
	display_name: 'DALL-E 3',
	kind: 'image'
});

export type ProbeStore = ReturnType<typeof createModelTestStore>;

export function probeFixture(): { stub: ModelStub; tests: ProbeStore } {
	return { stub: stubModels({ catalog: [chatRow, imageRow] }), tests: createModelTestStore() };
}

type RenderHandle = ReturnType<typeof render>;

let view: RenderHandle | undefined;

/** Renders the catalog and waits until its rows are on screen, not merely requested. */
export async function renderCatalog(tests: ProbeStore, token = 0): Promise<HTMLElement> {
	view = render(ModelCatalogList, {
		props: {
			providerId: 'openai',
			disabled: createModelDisabledStore(),
			thinking: createProviderThinkingStore(),
			tests,
			onchanged: () => {},
			token
		}
	});
	const container = view.container as HTMLElement;
	await waitFor(() => {
		expect(container.querySelector('tbody tr')).toBeTruthy();
	});
	return container;
}

/** Re-renders the last catalog mount, which is how the token-change case reads again. */
export function rerenderCatalog(props: { token: number }): void {
	view?.rerender(props);
}

export function rowOf(container: HTMLElement, modelId: string): HTMLElement {
	const rows = [...container.querySelectorAll('tbody tr')];
	const match = rows.find((row) => row.textContent?.includes(modelId));
	if (!match) throw new Error(`no catalog row for ${modelId}`);
	return match as HTMLElement;
}

export function testButton(row: HTMLElement): HTMLButtonElement {
	return within(row).getByRole('button', { name: /^Test/ }) as HTMLButtonElement;
}
