// Harness for the Custom provider card tests (docs/SPEC-UI/001-SPEC-UI.md §6.3).
//
// The card is mounted with a page-level reload, so a case can prove an edit reaches the provider read the
// page owns (§8.6.3) rather than only the node's own write.
//
// Two lookups are here because both cards and dialogs share this screen: `facts` scopes a fact to the
// card's own list, since the dialog mounted beside it repeats some of those values as option labels, and
// `dialogOf` scopes a sentence to the dialog that carries its heading.

import { fireEvent, render, screen } from '@testing-library/svelte';
import CustomProviderCard from '$lib/components/CustomProviderCard.svelte';
import type { NodeStub } from './provider-node-stub';

/** Renders the card with a page-level reload, so an edit can be shown to reach the provider read too. */
export function renderCard(
	stub: NodeStub,
	providerId = 'openai-compatible-01J'
): { changed: () => number } {
	let changed = 0;
	render(CustomProviderCard, {
		props: {
			providerId,
			onchanged: () => {
				changed += 1;
			}
		}
	});
	return { changed: () => changed };
}

export async function type(label: string, text: string): Promise<void> {
	await fireEvent.input(screen.getByLabelText(label), { target: { value: text } });
}

/**
 * The card's fact list, found through one of its own terms.
 *
 * The dialog is mounted beside the card and its option labels repeat some of these values, so a bare
 * `getByText` would match an option rather than the fact the card states.
 */
export function facts(): HTMLElement {
	return screen.getByText('Model prefix').closest('dl') as HTMLElement;
}

/** The dialog a heading belongs to, so a sentence is read from the dialog that carries it. */
export function dialogOf(heading: string): HTMLElement {
	return screen.getByRole('heading', { name: heading }).closest('dialog') as HTMLElement;
}
