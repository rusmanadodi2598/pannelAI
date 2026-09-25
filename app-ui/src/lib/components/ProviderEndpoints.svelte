<script lang="ts">
	// The provider-scoped endpoint list on the provider detail screen (docs/SPEC-UI/001-SPEC-UI.md §6.3).
	//
	// The scope is the route, not a control: the API takes `provider_id`, so the list is filtered by the
	// server and the panel never narrows a broader page in the browser. The drawer is the same one §6.2
	// uses, so an endpoint behaves identically wherever it is opened from.
	//
	// The section's own copy calls a row a connection, which is the reference's word for it and the one
	// the operator adds (`AddApiKeyModal.js` on that same screen); the table keeps the API's own word for
	// the row, because it is §6.2's table and the drawer behind it edits an endpoint.
	import { untrack } from 'svelte';
	import EndpointDetailDrawer from '$lib/components/EndpointDetailDrawer.svelte';
	import EndpointTable from '$lib/components/EndpointTable.svelte';
	import StateMessage from '$lib/components/StateMessage.svelte';
	import { listEndpoints } from '$lib/api/endpoints';
	import { CONTROL_ICONS } from '$lib/icons';
	import type { Endpoint } from '$lib/schemas/endpoint';

	let { providerId, token = 0 }: { providerId: string; token?: number } = $props();

	const PreviousIcon = CONTROL_ICONS.previous.icon;
	const NextIcon = CONTROL_ICONS.next.icon;

	const PAGE_SIZE = 25;

	let endpoints = $state<Endpoint[]>([]);
	let total = $state(0);
	let pageNumber = $state(1);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let selected = $state<Endpoint | null>(null);

	const lastPage = $derived(Math.max(1, Math.ceil(total / PAGE_SIZE)));

	// Reloads when the route's provider changes and when the token moves, which is how a key added from this
	// screen's own dialog reaches the table. Untracked for the same reason as the model catalog: the paging
	// handlers below reload explicitly, so a tracked read would fire a second, identical request.
	$effect(() => {
		const provider = providerId;
		const revision = token;
		untrack(() => void load(provider, revision));
	});

	async function load(provider: string, revision = token): Promise<void> {
		loading = true;
		const result = await listEndpoints({
			provider_id: provider,
			page: pageNumber,
			per_page: PAGE_SIZE
		});
		loading = false;

		// A newer load owns this state now, so an older answer that lands late is dropped rather than
		// overwriting it. `revision` is the token this call started with.
		if (revision !== token) return;

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
		<StateMessage kind="loading" title="Loading this provider's connections" />
	{:else if error}
		<StateMessage
			kind="error"
			title="This provider's connections could not be loaded"
			description={error}
		>
			{#snippet action()}
				<button type="button" class="underline" onclick={() => load(providerId)}>Try again</button>
			{/snippet}
		</StateMessage>
	{:else if endpoints.length === 0}
		<StateMessage
			kind="empty"
			title="No connections yet"
			description="A connection carries the credential the gateway routes this provider's calls with. Add one to start using it."
		/>
	{:else}
		<EndpointTable {endpoints} onopen={(entry) => (selected = entry)} />

		<div class="flex items-center gap-3 text-sm">
			<button
				type="button"
				class="inline-flex min-h-11 items-center gap-1 underline disabled:opacity-50"
				disabled={pageNumber <= 1}
				onclick={() => {
					pageNumber -= 1;
					void load(providerId);
				}}
			>
				<PreviousIcon class="size-4" aria-hidden="true" />
				Previous
			</button>
			<span class="text-[var(--color-text-muted)]">Page {pageNumber} of {lastPage}</span>
			<button
				type="button"
				class="inline-flex min-h-11 items-center gap-1 underline disabled:opacity-50"
				disabled={pageNumber >= lastPage}
				onclick={() => {
					pageNumber += 1;
					void load(providerId);
				}}
			>
				Next
				<NextIcon class="size-4" aria-hidden="true" />
			</button>
		</div>
	{/if}
</div>

<EndpointDetailDrawer
	entry={selected}
	onclose={() => (selected = null)}
	onchanged={() => load(providerId)}
/>
