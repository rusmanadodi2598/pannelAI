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
	//
	// Removing a connection lives here: a delete requested from a row or from the drawer opens one
	// confirmation this list owns, so the two paths cannot disagree about what a delete means. The
	// cascade (the endpoint's keys go with it) is stated in the dialog; here a success reloads the page's
	// own window and closes any drawer that was showing the row that left.
	import { untrack } from 'svelte';
	import EndpointDeleteDialog from '$lib/components/EndpointDeleteDialog.svelte';
	import EndpointDetailDrawer from '$lib/components/EndpointDetailDrawer.svelte';
	import EndpointTable from '$lib/components/EndpointTable.svelte';
	import StateMessage from '$lib/components/StateMessage.svelte';
	import { deleteEndpoint, listEndpoints } from '$lib/api/endpoints';
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

	// The connection awaiting confirmation, and the outcome of the delete attempt, held here so the
	// dialog's "Deleting" state and a refused delete both survive the round trip.
	let pendingDelete = $state<Endpoint | null>(null);
	let deleting = $state(false);
	let deleteError = $state<string | null>(null);

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

	function askDelete(entry: Endpoint): void {
		// A delete from the row closes any open drawer first, so the confirmation is the only modal on
		// screen and the row that leaves cannot stay selected behind it.
		selected = null;
		deleteError = null;
		pendingDelete = entry;
	}

	async function confirmDelete(): Promise<void> {
		if (!pendingDelete) return;

		deleting = true;
		const result = await deleteEndpoint(pendingDelete.id);
		deleting = false;

		if (!result.ok) {
			deleteError = result.error.message;
			return;
		}

		// The named row is gone; reload the window rather than splice it locally, so the total and the
		// paging the server owns stay truthful. Clearing the pending row closes the dialog.
		pendingDelete = null;
		deleteError = null;
		await load(providerId);
	}
</script>

<div class="flex flex-col gap-4">
	{#if loading}
		<StateMessage kind="loading" title="Loading this provider's connections" />
	{:else if error}
		<StateMessage
			kind="error"
			title="Could not load this provider's connections"
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
			description="Connections hold the credential used to route. Add one to start."
		/>
	{:else}
		<EndpointTable {endpoints} onopen={(entry) => (selected = entry)} ondelete={askDelete} />

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
	ondelete={askDelete}
/>

{#if pendingDelete}
	<EndpointDeleteDialog
		entry={pendingDelete}
		error={deleteError}
		{deleting}
		onconfirm={() => void confirmDelete()}
		oncancel={() => {
			pendingDelete = null;
			deleteError = null;
		}}
	/>
{/if}
