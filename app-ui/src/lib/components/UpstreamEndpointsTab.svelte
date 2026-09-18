<script lang="ts">
	// Upstream endpoints tab of Endpoint & Key (docs/SPEC-UI/001-SPEC-UI.md §6.2, tab 2).
	//
	// The tab owns the query, the filters, the create form's visibility, and the data states; the table and
	// the create form are their own components.
	//
	// The provider filter offers the providers present on the page being shown, because the registry list is
	// not this screen's data and §6.3 forbids pulling it in to filter. Choosing a provider is the Providers
	// screen's job, which is why the empty state links there and why the create form appears only once a
	// provider has been chosen: §6.3 arrives here with `?provider=` already set.
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import CreateEndpointForm from '$lib/components/CreateEndpointForm.svelte';
	import EndpointDetailDrawer from '$lib/components/EndpointDetailDrawer.svelte';
	import EndpointTable from '$lib/components/EndpointTable.svelte';
	import StateMessage from '$lib/components/StateMessage.svelte';
	import { listEndpoints } from '$lib/api/endpoints';
	import { ENDPOINT_STATUS_ACTIVE, type Endpoint } from '$lib/schemas/endpoint';
	import { onMount } from 'svelte';

	const PAGE_SIZE = 25;

	let endpoints = $state<Endpoint[]>([]);
	let total = $state(0);
	let page_ = $state(1);
	let loading = $state(true);
	let error = $state<string | null>(null);

	let providerFilter = $state('');
	let statusFilter = $state('');
	let selected = $state<Endpoint | null>(null);
	let creating = $state(true);

	const chosenProvider = $derived(page.url.searchParams.get('provider') ?? '');
	const lastPage = $derived(Math.max(1, Math.ceil(total / PAGE_SIZE)));

	// The API orders by priority already, but the panel does not assume it: a stable order is what makes the
	// priority column readable, so it sorts by priority and then by label.
	const ordered = $derived(
		[...endpoints].sort(
			(left, right) => left.priority - right.priority || left.label.localeCompare(right.label)
		)
	);

	const shown = $derived(
		ordered.filter(
			(entry) =>
				(providerFilter === '' || entry.provider_id === providerFilter) &&
				(statusFilter === '' || entry.status === statusFilter)
		)
	);

	const providers = $derived(
		[
			...new Map(
				endpoints.map((entry) => [entry.provider_id, entry.provider_name ?? entry.provider_id])
			).entries()
		]
			.map(([id, name]) => ({ id, name }))
			.sort((left, right) => left.name.localeCompare(right.name))
	);

	const filtered = $derived(providerFilter !== '' || statusFilter !== '');

	onMount(load);

	async function load(): Promise<void> {
		loading = true;
		const result = await listEndpoints({ page: page_, per_page: PAGE_SIZE });
		loading = false;

		if (!result.ok) {
			error = result.error.message;
			return;
		}

		error = null;
		endpoints = result.data.data;
		total = result.data.meta.total;
	}

	function clearFilters(): void {
		providerFilter = '';
		statusFilter = '';
	}
</script>

<div class="flex flex-col gap-5">
	{#if chosenProvider && creating}
		<div class="flex flex-col gap-2">
			<CreateEndpointForm
				providerId={chosenProvider}
				oncreated={() => {
					creating = false;
					void load();
				}}
			/>
			<button type="button" class="min-h-11 self-start underline" onclick={() => (creating = false)}
				>Hide this form</button
			>
		</div>
	{/if}

	<div class="flex flex-wrap items-end gap-3">
		<label class="flex flex-col gap-1 text-sm">
			<span class="text-[var(--color-text-muted)]">Provider</span>
			<select
				bind:value={providerFilter}
				class="min-h-11 rounded-[var(--radius-sm)] border border-[var(--color-border)] bg-[var(--color-surface)] px-2 text-sm"
			>
				<option value="">All providers</option>
				{#each providers as provider (provider.id)}
					<option value={provider.id}>{provider.name}</option>
				{/each}
			</select>
		</label>

		<label class="flex flex-col gap-1 text-sm">
			<span class="text-[var(--color-text-muted)]">Status</span>
			<select
				bind:value={statusFilter}
				class="min-h-11 rounded-[var(--radius-sm)] border border-[var(--color-border)] bg-[var(--color-surface)] px-2 text-sm"
			>
				<option value="">Any status</option>
				<option value={ENDPOINT_STATUS_ACTIVE}>Active</option>
				<option value="disabled">Disabled</option>
			</select>
		</label>

		{#if filtered}
			<button type="button" class="min-h-11 underline" onclick={clearFilters}>Clear filters</button>
		{/if}
	</div>

	{#if loading}
		<StateMessage kind="loading" title="Loading upstream endpoints" />
	{:else if error}
		<StateMessage kind="error" title="Upstream endpoints could not be loaded" description={error}>
			{#snippet action()}
				<button type="button" class="underline" onclick={load}>Try again</button>
			{/snippet}
		</StateMessage>
	{:else if endpoints.length === 0}
		<StateMessage
			kind="empty"
			title="No upstream endpoints yet"
			description="Pick a provider, then add an endpoint for it to start routing."
		>
			{#snippet action()}
				<a href={resolve('/providers')} class="underline">Choose a provider</a>
			{/snippet}
		</StateMessage>
	{:else if shown.length === 0}
		<StateMessage
			kind="empty"
			title="No endpoint matches these filters"
			description="Clear the filters to see the whole list again."
		>
			{#snippet action()}
				<button type="button" class="underline" onclick={clearFilters}>Clear filters</button>
			{/snippet}
		</StateMessage>
	{:else}
		<EndpointTable {endpoints} onopen={(entry) => (selected = entry)} />

		<div class="flex items-center gap-3 text-sm">
			<button
				type="button"
				class="underline disabled:opacity-50"
				disabled={page_ <= 1}
				onclick={() => {
					page_ -= 1;
					void load();
				}}>Previous</button
			>
			<span class="text-[var(--color-text-muted)]">Page {page_} of {lastPage}</span>
			<button
				type="button"
				class="underline disabled:opacity-50"
				disabled={page_ >= lastPage}
				onclick={() => {
					page_ += 1;
					void load();
				}}>Next</button
			>
		</div>
	{/if}
</div>

<EndpointDetailDrawer entry={selected} onclose={() => (selected = null)} onchanged={load} />
