<script lang="ts">
	// The provider detail model catalog (docs/SPEC-UI/001-SPEC-UI.md §6.3).
	//
	// The component owns its query, so the provider is the only thing passed in. Three parameters reach the
	// API: the provider scope, a capability filter, and free text. §6.3 also asks for a "suggested" toggle
	// and for per-model enable or disable state; the first is absent because the registry reports every
	// model as suggested, so the control would filter nothing (R-26), and the second writes through
	// `PUT /models/disabled`, which §6.3 itself places in U2.
	import { untrack } from 'svelte';
	import StateMessage from '$lib/components/StateMessage.svelte';
	import { listModelCatalog } from '$lib/api/models';
	import {
		CATALOG_CAPABILITY_FILTERS,
		catalogModelLabel,
		catalogQueryParams,
		catalogSourceLabel,
		type CatalogModel
	} from '$lib/schemas/model';

	let { providerId }: { providerId: string } = $props();

	let models = $state<CatalogModel[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);

	// `draft` is what the operator types; `applied` is what the API was asked. Keeping them apart is what
	// lets the search box hold a half-typed word without firing a request per keystroke.
	let draft = $state('');
	let applied = $state('');
	let capability = $state('');

	const filtered = $derived(applied !== '' || capability !== '');

	// Reloads when the route's provider changes. The filter state is read untracked because the handlers
	// below already reload on a filter change, and the reads inside `load` happen synchronously before its
	// first await: tracking them here would fire a second, identical request on every filter click.
	$effect(() => {
		const provider = providerId;
		untrack(() => void load(provider));
	});

	async function load(provider: string): Promise<void> {
		loading = true;
		const result = await listModelCatalog(
			catalogQueryParams({ providerId: provider, capability, query: applied })
		);
		loading = false;

		if (!result.ok) {
			error = result.error.message;
			return;
		}

		error = null;
		models = result.data.data;
	}

	function applySearch(): void {
		applied = draft;
		void load(providerId);
	}

	function toggleCapability(value: string): void {
		capability = capability === value ? '' : value;
		void load(providerId);
	}

	function clearFilters(): void {
		draft = '';
		applied = '';
		capability = '';
		void load(providerId);
	}
</script>

<div class="flex flex-col gap-4">
	<div class="flex flex-col gap-3">
		<form
			class="flex flex-wrap items-end gap-3"
			onsubmit={(event) => {
				event.preventDefault();
				applySearch();
			}}
		>
			<label class="flex flex-col gap-1 text-sm">
				<span class="text-[var(--color-text-muted)]">Search models</span>
				<input
					type="search"
					bind:value={draft}
					placeholder="Model id or name"
					class="min-h-11 w-64 rounded-[var(--radius-sm)] border border-[var(--color-border)] bg-[var(--color-surface)] px-2 text-sm"
				/>
			</label>
			<button type="submit" class="min-h-11 underline">Search</button>
		</form>

		<div class="flex flex-wrap items-center gap-2">
			<span class="text-sm text-[var(--color-text-muted)]">Capability</span>
			{#each CATALOG_CAPABILITY_FILTERS as option (option)}
				<button
					type="button"
					aria-pressed={capability === option}
					class="min-h-11 rounded-[var(--radius-sm)] border border-[var(--color-border)] px-3 text-sm aria-pressed:bg-[var(--color-surface-2)]"
					onclick={() => toggleCapability(option)}>{option}</button
				>
			{/each}
			{#if filtered}
				<button type="button" class="min-h-11 underline" onclick={clearFilters}
					>Clear filters</button
				>
			{/if}
		</div>
	</div>

	{#if loading}
		<StateMessage kind="loading" title="Loading the model catalog" />
	{:else if error}
		<StateMessage kind="error" title="The model catalog could not be loaded" description={error}>
			{#snippet action()}
				<button type="button" class="underline" onclick={() => load(providerId)}>Try again</button>
			{/snippet}
		</StateMessage>
	{:else if models.length === 0 && filtered}
		<StateMessage
			kind="empty"
			title="No models found for this filter"
			description="Clear the filters to see every model this provider offers."
		>
			{#snippet action()}
				<button type="button" class="underline" onclick={clearFilters}>Clear filters</button>
			{/snippet}
		</StateMessage>
	{:else if models.length === 0}
		<StateMessage
			kind="empty"
			title="This provider offers no models"
			description="The catalog reports nothing for this provider. A model the gateway has disabled is left out of the catalog, so check the disabled set before treating this as an empty registry entry."
		/>
	{:else}
		<div class="overflow-x-auto rounded-[var(--radius-md)] border border-[var(--color-border)]">
			<table class="w-full min-w-[44rem] border-collapse text-sm">
				<caption class="sr-only">Model catalog for this provider</caption>
				<thead class="bg-[var(--color-surface-2)] text-left">
					<tr>
						<th scope="col" class="px-3 py-2 font-medium">Model</th>
						<th scope="col" class="px-3 py-2 font-medium">Kind</th>
						<th scope="col" class="px-3 py-2 font-medium">Capabilities</th>
						<th scope="col" class="px-3 py-2 font-medium">Source</th>
					</tr>
				</thead>
				<tbody>
					{#each models as model (model.id)}
						<tr class="border-t border-[var(--color-border)]">
							<td class="px-3 py-2">
								<span class="font-medium">{catalogModelLabel(model)}</span>
								<br />
								<span class="text-[var(--color-text-muted)]">{model.model_id}</span>
							</td>
							<td class="px-3 py-2">{model.kind ?? 'Not declared'}</td>
							<td class="px-3 py-2">
								{#if model.capabilities.length === 0}
									<span class="text-[var(--color-text-muted)]">None declared</span>
								{:else}
									<span class="flex flex-wrap gap-1">
										{#each model.capabilities as entry (entry)}
											<span
												class="rounded-[var(--radius-sm)] bg-[var(--color-surface-2)] px-2 py-0.5"
												>{entry}</span
											>
										{/each}
									</span>
								{/if}
							</td>
							<td class="px-3 py-2">{catalogSourceLabel(model.source)}</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>

		<p class="text-sm text-[var(--color-text-muted)]">
			{models.length}
			{models.length === 1 ? 'model' : 'models'}
			{#if filtered}match these filters{:else}in this provider{/if}.
		</p>
	{/if}
</div>
