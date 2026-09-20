// The disabled model set as the provider detail screen uses it (docs/SPEC-UI/001-SPEC-UI.md §6.3).
//
// One screen, two halves: the catalog lists what the provider can route and offers Disable, and the
// disabled list shows what it cannot and offers Enable. Both write the same resource, so one store owns
// it. Two owners would each merge over their own last read, and the second write would put the first
// one's change back the way it was.
//
// The store holds the WHOLE set, every provider's rows, because `PUT /models/disabled` replaces the
// whole set: a write built from one provider's slice would delete every other provider's rows. The
// narrowed view is for display only.

import { listDisabledModels, replaceDisabledModels } from '$lib/api/models';
import {
	disabledRefKey,
	providerDisabledRefs,
	withDisabledRef,
	withoutDisabledRef,
	type DisabledRef
} from '$lib/schemas/model-disabled';

// What the last write did, and to which ref. The half of the screen that holds that ref renders it, so a
// failure appears beside the action that produced it and a success appears in the list the row moved to.
export type DisabledOutcome = {
	key: string;
	ok: boolean;
	message: string;
};

export function createModelDisabledStore() {
	let refs = $state<DisabledRef[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let saving = $state<string | null>(null);
	let outcome = $state<DisabledOutcome | null>(null);

	// `ready` is what makes a write safe. The merge runs over `refs`, so a write before the first
	// successful read would send an empty set and clear every provider's rows. A read that failed leaves
	// this false and the screen refuses to write until it is true.
	let ready = $state(false);

	async function load(): Promise<void> {
		loading = true;
		const result = await listDisabledModels();
		loading = false;

		if (!result.ok) {
			error = result.error.message;
			ready = false;
			return;
		}

		error = null;
		ready = true;
		refs = result.data.data;
	}

	// Applies one change to the whole set and takes the server's answer as the new state, so the panel
	// renders what was stored rather than what it hoped to store. Returns whether the write landed, which
	// is what tells the caller to refresh the catalog: every change here adds or removes a catalog row.
	async function setDisabled(ref: DisabledRef, disabled: boolean): Promise<boolean> {
		const key = disabledRefKey(ref);

		if (!ready) {
			outcome = {
				key,
				ok: false,
				message: 'The disabled set has not loaded, so this write is refused.'
			};
			return false;
		}

		saving = key;
		const next = disabled ? withDisabledRef(refs, ref) : withoutDisabledRef(refs, ref);
		const result = await replaceDisabledModels(next);
		saving = null;

		if (!result.ok) {
			outcome = { key, ok: false, message: result.error.message };
			return false;
		}

		error = null;
		refs = result.data.data;
		outcome = {
			key,
			ok: true,
			message: disabled ? `${key} is disabled.` : `${key} is routable again.`
		};
		return true;
	}

	return {
		get refs() {
			return refs;
		},
		get loading() {
			return loading;
		},
		get error() {
			return error;
		},
		// The ref a write is in flight for, or null. Every action is held while one is in flight: a second
		// write would merge over a set the first one is about to replace.
		get saving() {
			return saving;
		},
		get outcome() {
			return outcome;
		},
		get ready() {
			return ready;
		},
		mine(providerId: string): DisabledRef[] {
			return providerDisabledRefs(refs, providerId);
		},
		load,
		setDisabled
	};
}

export type ModelDisabledStore = ReturnType<typeof createModelDisabledStore>;
