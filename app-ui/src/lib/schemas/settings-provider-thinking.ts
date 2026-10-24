// The reasoning vocabulary and the per-provider thinking map (docs/SPEC-API/001-SPEC-API.md §7.14, §7.15).
import { z } from 'zod';

// The server accepts exactly these ten words: `on` and `off` are the two fixed states the reference
// injects, and the rest are level names, including `none` (thinking disabled) and `thinking` (z.ai's
// own word).
export const THINKING_MODES = [
	'on',
	'off',
	'none',
	'minimal',
	'low',
	'medium',
	'high',
	'xhigh',
	'max',
	'thinking'
] as const;

export type ThinkingMode = (typeof THINKING_MODES)[number];

// The picker's own word for following the request, which is the state a provider with no stored entry
// is in. It is not a storable mode: choosing it deletes the entry.
export const THINKING_AUTO = 'auto';

// The label a level renders with, on the reference's own rule (`page.js`): the word capitalized. `auto`
// is spelled out because "Auto" alone would not say what it follows.
export function thinkingModeLabel(mode: string): string {
	if (mode === THINKING_AUTO) return 'Auto (follow the request)';
	return mode.charAt(0).toUpperCase() + mode.slice(1);
}

// The suffix a copied model name gains when the control is set to a level that model accepts (SPEC-API
// §7.15, the reference's `resolveThinkingSuffix`): the gateway strips the group before resolving the
// model and reads it as this call's override. An empty string is the no-suffix answer, so a caller
// appends the result unconditionally.
export function thinkingSuffix(levels: readonly string[] | undefined, mode: string): string {
	if (mode === '' || mode === THINKING_AUTO) return '';
	if (levels === undefined || !levels.includes(mode)) return '';
	return `(${mode})`;
}

// One provider's stored reasoning mode. The gateway refuses an entry without a mode (an entry nobody
// can act on is dead configuration, `validateProviderThinking`), so a stored entry always names one and
// the read requires it: the write below sends the whole map back, and a mode the read could not name is
// one the panel could not forward either. The set is the same closed set on both sides, because a PATCH
// is refused by name for a mode outside it.
export const schemaProviderThinking = z.object({
	mode: z.enum(THINKING_MODES)
});

export const schemaReasoningSettings = z.object({
	// The server always answers an object (SPEC-API §7.14), so a document without the key
	// is drift rather than an older answer.
	provider_thinking: z.record(z.string(), schemaProviderThinking)
});

// The provider screen's reasoning control: one provider's entry inside the whole mode map (SPEC-API
// §7.14). Separate from the other patches for the same reason they are separate from each other: this
// one writes the map and must not carry a group it never rendered. The map is sent whole, so a caller
// read-modify-writes it, and deleting an entry is how a provider returns to following the client's own
// request.
export const schemaProviderThinkingPatch = z.strictObject({
	reasoning: z.strictObject({
		provider_thinking: z.record(
			z.string(),
			z.strictObject({
				mode: z.enum(THINKING_MODES, { message: 'Pick a reasoning mode the gateway accepts.' })
			})
		)
	})
});

export type ProviderThinking = z.infer<typeof schemaProviderThinking>;
export type ProviderThinkingPatch = z.infer<typeof schemaProviderThinkingPatch>;
