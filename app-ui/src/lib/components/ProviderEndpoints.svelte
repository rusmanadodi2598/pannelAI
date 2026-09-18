<script lang="ts">
	// The provider-scoped endpoint list on the provider detail screen (docs/SPEC-UI/001-SPEC-UI.md §6.3).
	//
	// The scope is the route, not a control: the API takes `provider_id`, so the list is filtered by the
	// server and the panel never narrows a broader page in the browser. The drawer is the same one §6.2
	// uses, so an endpoint behaves identically wherever it is opened from.
	import { resolve } from '$app/paths';
	import { untrack } from 'svelte';
	import EndpointDetailDrawer from '$lib/components/EndpointDetailDrawer.svelte';
	import EndpointTable from '$lib/components/EndpointTable.svelte';
	import StateMessage from '$lib/components/StateMessage.svelte';
	import { listEndpoints } from '$lib/api/endpoints';
	import type { Endpoint } from '$lib/schemas/endpoint';

	let { providerId }: { providerId: string } = $props();

	const PAGE_SIZE = 25;

	let endpoints = $state<Endpoint[]>([]);
	let total = $state(0);
	let pageNumber = $state(1);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let selected = $state<Endpoint | null>(null);

	const lastPage = $derived(Math.max(1, Math.ceil(total / PAGE_SIZE)));

	// Reloads when the route's provider changes. Untracked for the same reason as the model catalog: the
	// paging handlers below reload explicitly, so a tracked read would fire a second, identical request.
	$effect(() => {
		const provider = providerId;
		untrack(() => void load(provider));
	});

	async function load(provider: string): Promise<void> {
		loading = true;
		const result = await listEndpoints({
			provider_id: provider,
			page: pageNumber,
			per_page: PAGE_SIZE
		});
		loading = false;

		if (!result.ok) {
			error = result.error.message;
			return;
		}

		error = null;
		endpoints = result.data.data;
		total = result.data.meta.total;
	}
</script>

<div class="flex flex-col gap-4">
	{#if loading}
		<StateMessage kind="loading" title="Loading this provider's endpoints" />
	{:else if error}
		<StateMessage
			kind="error"
			title="This provider's endpoints could not be loaded"
			description={error}
		>
			{#snippet action()}
				<button type="button" class="underline" onclick={() => load(providerId)}>Try again</button>
			{/snippet}
		</StateMessage>
	{:else if endpoints.length === 0}
		<StateMessage
			kind="empty"
			title="No endpoint configured for this provider"
			description="An endpoint holds the credential the gateway routes with. Add one to start using this provider."
		>
			{#snippet action()}
				<a
					href={resolve(`/endpoint-keys?provider=${encodeURIComponent(providerId)}`)}
					class="underline">Add an endpoint</a
				>
			{/snippet}
		</StateMessage>
	{:else}
		<EndpointTable {endpoints} onopen={(entry) => (selected = entry)} />

		<div class="flex items-center gap-3 text-sm">
			<button
				type="button"
				class="underline disabled:opacity-50"
				disabled={pageNumber <= 1}
				onclick={() => {
					pageNumber -= 1;
					void load(providerId);
				}}>Previous</button
			>
			<span class="text-[var(--color-text-muted)]">Page {pageNumber} of {lastPage}</span>
			<button
				type="button"
				class="underline disabled:opacity-50"
				disabled={pageNumber >= lastPage}
				onclick={() => {
					pageNumber += 1;
					void load(providerId);
				}}>Next</button
			>
		</div>
	{/if}
</div>

<EndpointDetailDrawer
	entry={selected}
	onclose={() => (selected = null)}
	onchanged={() => load(providerId)}
/>
