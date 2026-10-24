// Harness for the Custom provider add controls.
//
// The section only creates nodes now; the registry table renders them after the page re-reads. Tests here
// mount the section with an `onsaved` counter so a case can prove the page reload happened after a write
// (§8.6.3), while the dialog's API calls go through the provided node stub.
//
// `fireEvent.input` rather than a hand-dispatched `Event('input')`: Svelte 5's `bind:value` ignores the
// bare event on these inputs, and a helper that silently left the draft untouched would make every case
// that fills a field pass for the wrong reason.

import { fireEvent, render, screen } from '@testing-library/svelte';
import CustomProviderSection from '$lib/components/CustomProviderSection.svelte';
import type { NodeStub } from './provider-node-stub';

/** Renders the add controls with a counter for the page reload the dialog asks for. */
export function renderSection(_stub: NodeStub): { reloads: () => number } {
	let reloads = 0;
	render(CustomProviderSection, {
		props: {
			onsaved: async () => {
				reloads += 1;
			}
		}
	});
	return { reloads: () => reloads };
}

/** Types into a field by its label, which is how the rest of this suite fills one. */
export async function type(label: string, text: string): Promise<void> {
	await fireEvent.input(screen.getByLabelText(label), { target: { value: text } });
}

export async function select(label: string, option: string): Promise<void> {
	await fireEvent.change(screen.getByLabelText(label), { target: { value: option } });
}
