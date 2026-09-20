<script lang="ts">
	// The RTK allowlist editor (docs/SPEC-UI/001-SPEC-UI.md §6.7, section 1).
	//
	// Twelve canonical filter names, one checkbox each, with the cap the filter applies stated under its
	// name so the choice is informed rather than a guess (SPEC-API-002 §5.1). The empty allowlist is the
	// state worth explaining: it stores as `[]` and means every filter is eligible, not none, so the
	// summary line says which of the two the current list is.
	//
	// A name the panel has no checkbox for is kept and named. The API cannot produce one today; a later
	// version could, and dropping it on save would change a stored configuration the operator never
	// touched.
	import { RTK_FILTERS } from '$lib/schemas/token-saver';
	import {
		rtkAllowlistLabel,
		toggleRtkFilter,
		unknownRtkFilters
	} from '$lib/schemas/token-saver-form';

	type Props = {
		filters: string[];
		onchange: (filters: string[]) => void;
	};

	let { filters, onchange }: Props = $props();

	const unknown = $derived(unknownRtkFilters(filters));
	const summary = $derived(rtkAllowlistLabel(filters));
</script>

<fieldset class="flex flex-col gap-2">
	<legend class="sr-only">RTK filters</legend>

	<p class="text-xs text-[var(--color-text-muted)]" role="status">{summary}</p>

	<ul class="flex flex-col gap-1">
		{#each RTK_FILTERS as filter (filter.name)}
			<li>
				<label class="flex items-start gap-3 text-sm">
					<input
						type="checkbox"
						class="mt-1 size-4"
						checked={filters.includes(filter.name)}
						onchange={(event) =>
							onchange(toggleRtkFilter(filters, filter.name, event.currentTarget.checked))}
					/>
					<span>
						<span class="font-medium">{filter.name}</span>
						<span class="block text-xs text-[var(--color-text-muted)]">{filter.description}</span>
					</span>
				</label>
			</li>
		{/each}
	</ul>

	{#if unknown.length > 0}
		<p class="text-xs text-[var(--color-warn)]">
			Stored but not offered here: {unknown.join(', ')}. They are kept as they are, because saving
			must not delete a filter this panel does not know.
		</p>
	{/if}
</fieldset>
