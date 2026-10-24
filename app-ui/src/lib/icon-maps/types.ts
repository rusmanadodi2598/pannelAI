// The shape every icon entry shares (docs/SPEC-UI/001-SPEC-UI.md §8.11).
//
// The component type is inferred from one installed glyph rather than restated: every icon in the set has
// this signature, and writing the library's generics by hand is how a set drifts from what it imports.
import { KeyRound } from '@lucide/svelte';

export type IconEntry = {
	icon: typeof KeyRound;
	reason: string;
};
