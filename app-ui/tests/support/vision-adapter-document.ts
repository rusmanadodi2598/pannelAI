// The vision adapter document fixture (docs/SPEC-API/001-SPEC-API.md §7.8).
//
// One row of the adapter tab: the flag, the round-robin switch, and the model list that is sent even when
// empty. Two schema files read it, the contract and the sentences the tab derives from it, and a third
// (`tests/components/vision-adapter-picker.test.ts`) renders the picker over it.

export function visionAdapterDocument(
	overrides: Record<string, unknown> = {}
): Record<string, unknown> {
	return {
		enabled: true,
		round_robin: false,
		models: ['openai/gpt-4o'],
		updated_at: '2026-09-18T00:00:00Z',
		...overrides
	};
}
