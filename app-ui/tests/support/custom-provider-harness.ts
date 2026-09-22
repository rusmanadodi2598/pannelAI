// Harness for the Custom provider section tests (docs/SPEC-UI/001-SPEC-UI.md §6.3).
//
// The section is the unit under test rather than the page: the page's own composition cases live in
// tests/routes/providers-custom-section.test.ts. What a test here needs is the section mounted with a
// node set the stub owns, plus a reload that re-reads through the same stub, so a case can prove the
// screen re-read after a write (§8.6.3) rather than trusting a render.
//
// `fireEvent.input` rather than a hand-dispatched `Event('input')`: Svelte 5's `bind:value` ignores the
// bare event on these inputs, and a helper that silently left the draft untouched would make every case
// that fills a field pass for the wrong reason.

import { fireEvent, render, screen } from '@testing-library/svelte';
import CustomProviderSection from '$lib/components/CustomProviderSection.svelte';
import type { ProviderNode } from '$lib/schemas/provider-node';
import type { NodeStub } from './provider-node-stub';

/** Renders the section as the page does, with a reload that re-reads through the same stub. */
export function renderSection(
	stub: NodeStub,
	overrides: { loading?: boolean; error?: string | null } = {}
): { reloads: () => number } {
	let reloads = 0;
	render(CustomProviderSection, {
		props: {
			// The stub keeps plain records because it mutates them the way the server does; the component
			// takes the shape the client parsed, which is what these records stand in for.
			nodes: stub.nodes as unknown as ProviderNode[],
			loading: overrides.loading ?? false,
			error: overrides.error ?? null,
			onreload: async () => {
				reloads += 1;
				await fetch('/api/v1/provider-nodes');
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
