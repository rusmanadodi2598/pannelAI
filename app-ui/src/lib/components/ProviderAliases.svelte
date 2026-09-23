<script lang="ts">
	// The alias set (docs/SPEC-UI/001-SPEC-UI.md §6.3).
	//
	// The set is global: it is not scoped to a provider, so this section takes no provider prop and every
	// provider's detail screen renders the same table. §6.3 is where it lives, and it is the only screen it
	// has: the nav list is fixed (§2.1), so no screen was added for it.
	//
	// The target suggestions come from two reads, the catalog (every provider's models) and the combo names,
	// because a target may be either. They are suggestions only: the API resolves the target, so a failed
	// read here leaves the form usable and the note says what happened.
	import { untrack } from 'svelte';
	import ModelAliasForm from '$lib/components/ModelAliasForm.svelte';
	import StateMessage from '$lib/components/StateMessage.svelte';
	import { listCombos } from '$lib/api/combos';
	import { listModelCatalog } from '$lib/api/models';
	import { createModelAliasStore } from '$lib/stores/model-alias.svelte';

	// The API caps any list at 100 entries (schema.MaxPerPage), so one request is the most the panel can ask
	// for. A longer list is truncated, and the note below says so rather than letting the picker look whole.
	const COMBO_SUGGESTION_LIMIT = 100;

	const aliases = createModelAliasStore();

	let suggestions = $state<string[]>([]);
	let suggestionNote = $state<string | null>(null);

	$effect(() => {
		untrack(() => void aliases.load());
	});

	$effect(() => {
		untrack(() => void loadSuggestions());
	});

	async function loadSuggestions(): Promise<void> {
		const [catalog, combos] = await Promise.all([
			listModelCatalog(),
			listCombos({ per_page: COMBO_SUGGESTION_LIMIT })
		]);

		if (!catalog.ok || !combos.ok) {
			suggestions = [];
			suggestionNote =
				'The suggestions could not be read, so a target has to be typed. The API still resolves it.';
			return;
		}

		// One list, deduplicated: a combo name may also be a catalog id, and the same string twice is noise
		// in a picker.
		suggestions = [
			...new Set([
				...catalog.data.data.map((row) => row.id),
				...combos.data.data.map((combo) => combo.name)
			])
		].sort((left, right) => (left < right ? -1 : left > right ? 1 : 0));

		const read = combos.data.data.length;
		const total = combos.data.meta.total;
		suggestionNote =
			total > read
				? `The combo names listed are the first ${read} of ${total}; a target beyond them can still be typed.`
				: null;
	}
</script>

<div class="flex flex-col gap-4">
	{#if aliases.loading}
		<StateMessage kind="loading" title="Loading the alias set" />
	{:else if aliases.error}
		<StateMessage
			kind="error"
			title="The alias set could not be loaded"
			description={aliases.error}
		>
			{#snippet action()}
				<button type="button" class="underline" onclick={() => aliases.load()}>Try again</button>
			{/snippet}
		</StateMessage>
	{:else if aliases.entries.length === 0}
		<StateMessage
			kind="empty"
			title="No aliases yet"
			description="An alias is a short name a client can send in place of a model string. Its target is a provider/model reference or a combo name."
		/>
	{:else}
		<!-- relative keeps the sr-only column label (position: absolute) inside this scroll box. -->
		<div
			class="relative overflow-x-auto rounded-[var(--radius-md)] border border-[var(--color-border)]"
		>
			<table class="w-full min-w-[36rem] border-collapse text-sm">
				<caption class="sr-only">The alias set, every alias the gateway resolves</caption>
				<thead class="bg-[var(--color-surface-2)] text-left">
					<tr>
						<th scope="col" class="px-3 py-2 font-medium">Alias</th>
						<th scope="col" class="px-3 py-2 font-medium">Target</th>
						<th scope="col" class="px-3 py-2 font-medium">
							<span class="sr-only">Actions</span>
						</th>
					</tr>
				</thead>
				<tbody>
					{#each aliases.entries as entry (entry.alias)}
						<tr class="border-t border-[var(--color-border)]">
							<td class="px-3 py-2 font-medium">{entry.alias}</td>
							<td class="px-3 py-2">{entry.target}</td>
							<td class="px-3 py-2 text-end">
								<button
									type="button"
									class="min-h-11 underline disabled:opacity-50"
									disabled={aliases.saving !== null}
									onclick={() => void aliases.remove(entry.alias)}>Remove</button
								>
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>

		<p class="text-sm text-[var(--color-text-muted)]">
			{aliases.entries.length}
			{aliases.entries.length === 1 ? 'alias' : 'aliases'}. The set is global: every provider's
			detail screen shows this same table, and a save writes all of it.
		</p>
	{/if}

	<!-- One line for the last write, because a removal leaves no row to carry its message: the row it names
	     is the one that is gone. -->
	{#if aliases.outcome}
		<p
			class="text-sm"
			role={aliases.outcome.ok ? 'status' : 'alert'}
			class:text-[var(--color-danger)]={!aliases.outcome.ok}
		>
			{aliases.outcome.message}
		</p>
	{/if}

	<!-- The form waits for the set: a write while the read is in error is refused by the store anyway, and a
	     form that looked usable would invite a write the panel cannot merge. -->
	{#if !aliases.loading && !aliases.error}
		<ModelAliasForm
			{suggestions}
			note={suggestionNote}
			busy={aliases.saving !== null}
			onadd={aliases.add}
		/>
	{/if}
</div>
