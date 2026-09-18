<script lang="ts">
	// Provider registry list (docs/SPEC-UI/001-SPEC-UI.md §6.3).
	//
	// Read-only: the registry is embedded and the API owns it, so this screen answers what the gateway can
	// route to and whether it has a working account, and nothing here writes. The category filter is the one
	// the API accepts; §6.3 also asks for a search over name and id, which the API does not take a parameter
	// for yet, so the control is absent rather than fake.
	import ProviderTable from '$lib/components/ProviderTable.svelte';
	import StateMessage from '$lib/components/StateMessage.svelte';
	import { listProviders } from '$lib/api/providers';
	import { PROVIDER_CATEGORIES, type Provider } from '$lib/schemas/provider';
	import { onMount } from 'svelte';

	const PAGE_SIZE = 25;

	let providers = $state<Provider[]>([]);
	let total = $state(0);
	let page = $state(1);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let category = $state('');

	const lastPage = $derived(Math.max(1, Math.ceil(total / PAGE_SIZE)));

	onMount(load);

	async function load(): Promise<void> {
		loading = true;
		const result = await listProviders({
			page,
			per_page: PAGE_SIZE,
			category: category === '' ? undefined : category
		});
		loading = false;

		if (!result.ok) {
			error = result.error.message;
			return;
		}

		error = null;
		providers = result.data.data;
		total = result.data.meta.total;
	}

	function applyCategory(value: string): void {
		category = value;
		page = 1;
		void load();
	}
</script>

<section class="flex flex-col gap-5">
	<div class="flex flex-col gap-1">
		<h1 class="text-lg font-semibold tracking-tight">Provider</h1>
		<p class="text-sm text-[var(--color-text-muted)]">
			What the gateway can route to, and how the stored accounts for each provider are doing.
		</p>
	</div>

	<label class="flex w-fit flex-col gap-1 text-sm">
		<span class="text-[var(--color-text-muted)]">Category</span>
		<select
			value={category}
			onchange={(event) => applyCategory(event.currentTarget.value)}
			class="min-h-11 rounded-[var(--radius-sm)] border border-[var(--color-border)] bg-[var(--color-surface)] px-2 text-sm"
		>
			<option value="">All categories</option>
			{#each PROVIDER_CATEGORIES as option (option)}
				<option value={option}>{option}</option>
			{/each}
		</select>
	</label>

	{#if loading}
		<StateMessage kind="loading" title="Loading the provider registry" />
	{:else if error}
		<StateMessage kind="error" title="The provider registry could not be loaded" description={error}>
			{#snippet action()}
				<button type="button" class="underline" onclick={load}>Try again</button>
			{/snippet}
		</StateMessage>
	{:else if providers.length === 0}
		<StateMessage
			kind="empty"
			title={category === '' ? 'The registry is empty' : `No provider in the ${category} category`}
			description={category === ''
				? 'The embedded registry reported no providers, which means the service was built without one.'
				: 'Clear the category filter to see the whole registry again.'}
		>
			{#snippet action()}
				{#if category !== ''}
					<button type="button" class="underline" onclick={() => applyCategory('')}
						>Clear the filter</button
					>
				{/if}
			{/snippet}
		</StateMessage>
	{:else}
		<ProviderTable {providers} />

		<div class="flex items-center gap-3 text-sm">
			<button
				type="button"
				class="underline disabled:opacity-50"
				disabled={page <= 1}
				onclick={() => {
					page -= 1;
					void load();
				}}>Previous</button
			>
			<span class="text-[var(--color-text-muted)]">Page {page} of {lastPage}</span>
			<button
				type="button"
				class="underline disabled:opacity-50"
				disabled={page >= lastPage}
				onclick={() => {
					page += 1;
					void load();
				}}>Next</button
			>
		</div>
	{/if}
</section>
