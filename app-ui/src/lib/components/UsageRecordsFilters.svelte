<script lang="ts">
	// The Usage Records filter bar (docs/SPEC-UI/001-SPEC-UI.md §6.5, tab 2).
	//
	// The two selects commit on change, because a select has no half-typed state to protect. The three
	// free-text filters are drafts until the operator submits them, so a half-typed model string does not
	// fire a request per keystroke.
	//
	// The component does not own the applied values: they come from the URL, and `synced` records which
	// applied state the boxes were last filled from, so a back or forward navigation moves the boxes as well
	// as the table while typing is never overwritten by the mirror.
	import { REQUEST_STATUSES, REQUEST_STATUS_LABELS } from '$lib/schemas/primitives';
	import { USAGE_PERIODS, USAGE_PERIOD_LABELS } from '$lib/schemas/usage';
	import { usageFiltersApplied, type UsageSearch } from '$lib/schemas/usage-search';

	type Props = {
		search: UsageSearch;
		/** Commits one filter, for the controls that have nothing to draft. */
		onchange: (key: string, value: string) => void;
		/** Commits the three text filters together, so one submit is one navigation. */
		onapply: (values: { endpointId: string; model: string; query: string }) => void;
		onclear: () => void;
	};

	let { search, onchange, onapply, onclear }: Props = $props();

	let drafts = $state({ endpoint: '', model: '', query: '' });
	let synced = $state('');

	$effect(() => {
		const key = `${search.endpointId}\n${search.model}\n${search.query}`;
		if (key === synced) return;

		synced = key;
		drafts = { endpoint: search.endpointId, model: search.model, query: search.query };
	});

	const filtered = $derived(usageFiltersApplied(search));

	const fieldClass =
		'min-h-11 rounded-[var(--radius-sm)] border border-[var(--color-border)] bg-[var(--color-surface)] px-2 text-sm';
</script>

<form
	class="flex flex-wrap items-end gap-3"
	onsubmit={(event) => {
		event.preventDefault();
		onapply({ endpointId: drafts.endpoint, model: drafts.model, query: drafts.query });
	}}
>
	<label class="flex flex-col gap-1 text-sm">
		<span class="text-[var(--color-text-muted)]">Period</span>
		<select
			value={search.period}
			onchange={(event) => onchange('period', event.currentTarget.value)}
			class={fieldClass}
		>
			{#each USAGE_PERIODS as period (period)}
				<option value={period}>{USAGE_PERIOD_LABELS[period]}</option>
			{/each}
		</select>
	</label>

	<label class="flex flex-col gap-1 text-sm">
		<span class="text-[var(--color-text-muted)]">Status</span>
		<select
			value={search.status}
			onchange={(event) => onchange('status', event.currentTarget.value)}
			class={fieldClass}
		>
			<option value="">Any status</option>
			{#each REQUEST_STATUSES as status (status)}
				<option value={status}>{REQUEST_STATUS_LABELS[status]}</option>
			{/each}
		</select>
	</label>

	<label class="flex flex-col gap-1 text-sm">
		<span class="text-[var(--color-text-muted)]">Endpoint id</span>
		<input bind:value={drafts.endpoint} placeholder="ep_..." class={`w-48 ${fieldClass}`} />
	</label>

	<label class="flex flex-col gap-1 text-sm">
		<span class="text-[var(--color-text-muted)]">Model</span>
		<input bind:value={drafts.model} placeholder="gpt-4o" class={`w-40 ${fieldClass}`} />
	</label>

	<label class="flex flex-col gap-1 text-sm">
		<span class="text-[var(--color-text-muted)]">Search</span>
		<input
			type="search"
			bind:value={drafts.query}
			placeholder="Request id, error code"
			class={`w-48 ${fieldClass}`}
		/>
	</label>

	<button
		type="submit"
		class="min-h-11 rounded-[var(--radius-sm)] border border-[var(--color-border)] px-3 text-sm"
		>Apply filters</button
	>

	{#if filtered}
		<button type="button" class="min-h-11 underline" onclick={onclear}>Clear filters</button>
	{/if}
</form>
