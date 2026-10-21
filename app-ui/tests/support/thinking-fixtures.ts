// The reasoning fixtures the two provider-thinking test files share
// (docs/SPEC-API/001-SPEC-API.md §7.14; docs/SPEC-UI/001-SPEC-UI.md §6.3).
//
// The mode is one settings value per provider, so both files build the same settings document and read it
// through the same provider id; a second copy of either would let the picker's cases and the page's wiring
// cases disagree about where the value lives.

import { reasoningGroup, settingsDocument } from './settings-document';

export const PROVIDER = 'openai';
export const LEVELS = ['low', 'medium', 'high', 'max'];

/** The settings document with this provider's stored mode, or another provider's. */
export function storedThinking(entries: Record<string, unknown> = {}): Record<string, unknown> {
	return settingsDocument({ reasoning: reasoningGroup({ provider_thinking: entries }) });
}
