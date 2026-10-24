// Whether a settings draft still differs from what the gateway holds (docs/SPEC-UI/001-SPEC-UI.md §6.13).
//
// Every settings tab asks the same question of its own group, and each tab's answer drives both its
// unsaved marker and its discard action, so the two cannot disagree about what changed.

/**
 * Whether one draft value equals the loaded value.
 *
 * A nested group such as `key_park` lives in its own object, so a reference compare would call it dirty
 * on every render even when nothing changed. Objects are compared by their entries instead; every scalar
 * still falls through to `===`.
 */
function sameValue(loaded: unknown, draft: unknown): boolean {
	if (loaded === draft) return true;

	if (loaded === null || draft === null) return false;
	if (typeof loaded !== 'object' || typeof draft !== 'object') return false;

	const left = loaded as Record<string, unknown>;
	const right = draft as Record<string, unknown>;
	const names = Object.keys(left);
	if (names.length !== Object.keys(right).length) return false;

	return names.every((name) => sameValue(left[name], right[name]));
}

/**
 * Whether a draft group differs from the group that was loaded.
 *
 * It compares by key rather than by position, so a reordered object is not dirty, and it treats a
 * missing loaded group as dirty because there is nothing to compare a draft against.
 */
export function settingsGroupDirty(
	loaded: Record<string, unknown> | null,
	draft: Record<string, unknown>
): boolean {
	if (loaded === null) return true;

	const keys = new Set([...Object.keys(loaded), ...Object.keys(draft)]);
	for (const key of keys) {
		if (!sameValue(loaded[key], draft[key])) return true;
	}

	return false;
}
