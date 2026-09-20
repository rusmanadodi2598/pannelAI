// The alias set as the provider detail screen uses it (docs/SPEC-UI/001-SPEC-UI.md §6.3).
//
// The set is global: it is not scoped to a provider, so this store has no provider in it and the table it
// feeds is the same on every provider's detail screen. Every change is a whole-set PUT, because that is
// the only write route the API has, so each write is a merge over the whole last read rather than a delta.
//
// Two rules come from the resource's own shape and are what the tests below hold the store to. A write
// before the first successful read would merge over an empty set and clear every alias in the gateway, so
// it is refused. And the answer to a write is the set the API stored, which is what the screen renders:
// the panel never reconstructs the set it hoped to store.

import { listModelAliases, replaceModelAliases } from '$lib/api/models';
import {
	aliasKey,
	withAlias,
	withoutAlias,
	type ModelAlias,
	type ModelAliasEntry
} from '$lib/schemas/model-alias';

// What the last write did, and to which alias. One line under the table renders it, because a removal
// leaves no row to render it in: the row it names is the one that is gone.
export type AliasOutcome = {
	name: string;
	ok: boolean;
	message: string;
};

export function createModelAliasStore() {
	let entries = $state<ModelAlias[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let saving = $state<string | null>(null);
	let outcome = $state<AliasOutcome | null>(null);

	// `ready` is what makes a write safe, for the reason the file header gives: a merge over a set that was
	// never read sends an empty set. A failed read leaves this false and every write is refused until a
	// read succeeds.
	let ready = $state(false);

	async function load(): Promise<void> {
		loading = true;
		const result = await listModelAliases();
		loading = false;

		if (!result.ok) {
			error = result.error.message;
			ready = false;
			return;
		}

		error = null;
		ready = true;
		entries = result.data.data;
	}

	// The one write path, so both actions hold the same rules: refuse before the first read, hold while a
	// write is in flight, and take the server's answer as the state.
	async function write(next: ModelAliasEntry[], name: string, done: string): Promise<boolean> {
		if (!ready) {
			outcome = {
				name,
				ok: false,
				message: 'The alias set has not loaded, so this write is refused.'
			};
			return false;
		}

		saving = name;
		const result = await replaceModelAliases(next);
		saving = null;

		if (!result.ok) {
			outcome = { name, ok: false, message: result.error.message };
			return false;
		}

		error = null;
		entries = result.data.data;
		outcome = { name, ok: true, message: done };
		return true;
	}

	// Adds an alias, or changes what an existing one targets. The message says which of the two happened,
	// so an operator who typed a name that was already here is told what their write did rather than left to
	// notice it in the table.
	async function add(entry: ModelAliasEntry): Promise<boolean> {
		const name = aliasKey(entry);
		const existed = entries.some((row) => aliasKey(row) === name);
		const next = withAlias(entries, entry);
		return write(
			next,
			name,
			existed ? `${name} now targets ${entry.target.trim()}.` : `${name} was added.`
		);
	}

	async function remove(name: string): Promise<boolean> {
		const key = name.trim();
		return write(withoutAlias(entries, key), key, `${key} no longer resolves.`);
	}

	return {
		get entries() {
			return entries;
		},
		get loading() {
			return loading;
		},
		get error() {
			return error;
		},
		// The alias a write is in flight for, or null. The screen holds its controls while one is in flight,
		// because a second write would merge over a set the first one is about to replace.
		get saving() {
			return saving;
		},
		get outcome() {
			return outcome;
		},
		get ready() {
			return ready;
		},
		load,
		add,
		remove
	};
}

export type ModelAliasStore = ReturnType<typeof createModelAliasStore>;
