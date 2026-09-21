<script lang="ts">
	// The upstream endpoints filter bar (docs/SPEC-UI/001-SPEC-UI.md §6.2, tab 2).
	//
	// Both selects commit on change, because a select has no half-typed state to protect. The component
	// does not own the applied values: they come from the URL, and the tab hands them down, so a back or
	// forward navigation moves the controls and the table together.
	//
	// The notices live here rather than in the tab because they are about the address the operator is
	// looking at: one names a filter value the panel had to correct, the other says the provider list is
	// short because its own read failed.
	import { ENDPOINT_STATUS_ACTIVE, ENDPOINT_STATUS_DISABLED } from '$lib/schemas/endpoint';
	import type { ProviderOption } from '$lib/schemas/endpoint-options';
	import { endpointFiltersApplied, type EndpointSearch } from '$lib/schemas/endpoint-search';

	type Props = {
		search: EndpointSearch;
		/** The providers the filter may offer, with the one the URL names always present. */
		options: ProviderOption[];
		/** The reason the option list is short, when its read failed. */
		optionsNotice: string | null;
		onchange: (key: string, value: string) => void;
		onclear: () => void;
	};

	let { search, options, optionsNotice, onchange, onclear }: Props = $props();

	const STATUS_LABELS: Record<string, string> = {
		[ENDPOINT_STATUS_ACTIVE]: 'Active',
		[ENDPOINT_STATUS_DISABLED]: 'Disabled'
	};

	const filtered = $derived(endpointFiltersApplied(search));

	const fieldClass =
		'min-h-11 rounded-[var(--radius-sm)] border border-[var(--color-border)] bg-[var(--color-surface)] px-2 text-sm';
</script>

<div class="flex flex-wrap items-end gap-3">
	<label class="flex flex-col gap-1 text-sm">
		<span class="text-[var(--color-text-muted)]">Provider</span>
		<select
			value={search.providerId}
			onchange={(event) => onchange('provider_id', event.currentTarget.value)}
			class={fieldClass}
		>
			<option value="">All providers</option>
			{#each options as provider (provider.id)}
				<option value={provider.id}>{provider.name}</option>
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
			<option value={ENDPOINT_STATUS_ACTIVE}>{STATUS_LABELS[ENDPOINT_STATUS_ACTIVE]}</option>
			<option value={ENDPOINT_STATUS_DISABLED}>{STATUS_LABELS[ENDPOINT_STATUS_DISABLED]}</option>
		</select>
	</label>

	{#if filtered}
		<button type="button" class="min-h-11 underline" onclick={onclear}>Clear filters</button>
	{/if}
</div>

{#if search.notices.length > 0}
	<div
		role="status"
		aria-live="polite"
		class="flex flex-col gap-1 rounded-[var(--radius-md)] bg-[var(--color-surface-2)] px-3 py-2 text-sm"
	>
		{#each search.notices as notice (notice)}
			<p>{notice}</p>
		{/each}
	</div>
{/if}

{#if optionsNotice}
	<p role="status" class="text-sm text-[var(--color-text-muted)]">{optionsNotice}</p>
{/if}
