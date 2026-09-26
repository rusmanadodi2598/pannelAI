// The reasoning mode the provider detail screen reads and writes (docs/SPEC-API/001-SPEC-API.md §7.14,
// §7.15; docs/SPEC-UI/001-SPEC-UI.md §6.3).
//
// One screen, three readers: the picker shows the mode and writes it, and the two model tables append
// the `(level)` suffix to the string they copy. All three have to read the same value, so one store owns
// it; a second owner would let a copied name carry a suffix the picker does not show.
//
// The store holds the mode the server stored for one provider, not the whole map: the write is the API
// module's own read-modify-write of the map (`patchProviderThinking`), so the map itself never needs to
// live here. That is also why the map's other providers' entries cannot be clobbered from this screen.
//
// `modeFor` answers only for the provider the last read or write named. That is what keeps a stale
// answer from rendering under the next provider: a route change reloads, and until the new answer lands
// this screen's tables copy no suffix rather than the previous provider's.

import { fetchSettings, patchProviderThinking } from '$lib/api/settings';
import type { ThinkingMode } from '$lib/schemas/settings';

export function createProviderThinkingStore() {
	// The stored mode, and the provider it was stored for. Empty is not a mode: it is the absent entry,
	// which is what follows the reasoning setting each request carries.
	let storedMode = $state('');
	let storedFor = $state('');
	let loading = $state(true);
	let error = $state<string | null>(null);
	let saving = $state(false);
	let outcome = $state<string | null>(null);

	// The id the latest read asked for, so an answer for a provider the screen has left is dropped
	// rather than rendered as this provider's mode.
	let requested = '';

	async function load(id: string): Promise<void> {
		requested = id;
		loading = true;
		// The last write's announcement described the provider this read is replacing, so it goes with
		// it rather than rendering under a picker it does not describe.
		outcome = null;
		const result = await fetchSettings();

		// A newer read owns this state now, so an older answer that lands late is dropped.
		if (id !== requested) return;
		loading = false;

		if (!result.ok) {
			error = result.error.message;
			storedMode = '';
			storedFor = '';
			return;
		}

		error = null;
		storedMode = result.data.reasoning.provider_thinking[id]?.mode ?? '';
		storedFor = id;
	}

	/**
	 * Writes one provider's entry and takes the server's answer as the new state, so the screen renders
	 * what was stored rather than what it hoped to store. Returns whether the write landed, which is
	 * what tells the picker to keep its choice or put it back.
	 */
	async function setMode(id: string, next: ThinkingMode | null): Promise<boolean> {
		outcome = null;
		saving = true;
		const result = await patchProviderThinking(id, next);
		saving = false;

		if (!result.ok) {
			outcome = result.error.message;
			return false;
		}

		storedMode = result.data.reasoning.provider_thinking[id]?.mode ?? '';
		storedFor = id;
		outcome =
			next === null
				? 'Follows the reasoning setting each request carries.'
				: `This provider now asks for ${next} on every request.`;
		return true;
	}

	return {
		/** The mode stored for `id`, or an empty string when none is. */
		modeFor(id: string): string {
			return storedFor === id ? storedMode : '';
		},
		get loading() {
			return loading;
		},
		get error() {
			return error;
		},
		get saving() {
			return saving;
		},
		get outcome() {
			return outcome;
		},
		load,
		setMode
	};
}

export type ProviderThinkingStore = ReturnType<typeof createProviderThinkingStore>;
