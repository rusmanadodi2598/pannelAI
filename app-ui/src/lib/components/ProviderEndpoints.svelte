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
	import BulkActionBar from '$lib/components/BulkActionBar.svelte';
	import EndpointTable from '$lib/components/EndpointTable.svelte';
	import ApiErrorText from '$lib/components/ApiErrorText.svelte';
	import PagingControls from '$lib/components/PagingControls.svelte';
	import StateMessage from '$lib/components/StateMessage.svelte';
	import type { ApiError } from '$lib/api/errors';
	import { deleteEndpoint, listEndpoints } from '$lib/api/endpoints';
	import { Button } from '$lib/primitives/button/index.js';
	import { createBulkActions } from '$lib/stores/bulk-actions.svelte';
	import type { Endpoint } from '$lib/schemas/endpoint';

	let { providerId, token = 0 }: { providerId: string; token?: number } = $props();

	// Ten connections a page. A provider can hold one per account, and the server pages the read, so the
	// page size is what keeps the list short on a phone.
	const PAGE_SIZE = 10;

	let endpoints = $state<Endpoint[]>([]);
	let total = $state(0);
	let pageNumber = $state(1);
	let loading = $state(true);
	let error = $state<ApiError | null>(null);
	let selected = $state<Endpoint | null>(null);

	// The connection awaiting confirmation, and the outcome of the delete attempt, held here so the
	// dialog's "Deleting" state and a refused delete both survive the round trip.
	let pendingDelete = $state<Endpoint | null>(null);
	let deleting = $state(false);
	let deleteError = $state<ApiError | null>(null);

	const lastPage = $derived(Math.max(1, Math.ceil(total / PAGE_SIZE)));

	// Bulk selection is a set of ids plus the server's count, never the ten rows this page shows:
	// Select-All sends `all` with the provider scope and the server resolves the 47, so the number
	// in the bar is the number the request will act on. `confirm_count` carries that same figure,
	// which is what turns a list that changed under the click into a refusal instead of a surprise.
	const bulk = createBulkActions({
		kind: 'endpoints',
		ids: () => endpoints.map((entry) => entry.id),
		total: () => total,
		scope: () => ({ provider_id: providerId }),
		onapplied: () => load(providerId)
	});

	let bulkNotice = $state<string | null>(null);

	const bulkScope = $derived(
		bulk.selected.all
			? `all ${total} connections at ${providerId}`
			: `${bulk.selected.ids.length} of the ${endpoints.length} on this page`
	);

	// The store reports how many rows changed; the sentence that names them connections is this screen's.
	async function runBulkStatus(status: 'active' | 'disabled'): Promise<void> {
		const done = await bulk.setStatus(status);
		if (!done) return;
		bulkNotice = `${done.changed} connection(s) ${status === 'disabled' ? "parked; they return on their own when the provider's reset window runs out" : 'returned to rotation'}.`;
	}

	async function runBulkDelete(): Promise<void> {
		const done = await bulk.remove();
		if (done) bulkNotice = `Removed ${done.changed} connection(s) and their stored keys.`;
	}

	// The bar hands over a tag it already normalized; the server canonicalizes once more and refuses
	// a label too long to store rather than cutting it.
	async function runBulkTag(tag: string): Promise<void> {
		const done = await bulk.tag(tag);
		if (done) bulkNotice = `Tagged ${done.changed} connection(s) "${tag}".`;
	}

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
			error = result.error;
			return;
		}

		error = null;
		endpoints = result.data.data;
		total = result.data.meta.total;
	}

	// The window is the server's, so a page step re-reads at the new offset instead of slicing what is
	// already on screen.
	function goPage(page: number): void {
		pageNumber = page;
		void load(providerId);
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
			deleteError = result.error;
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
		<StateMessage kind="error" title="Could not load this provider's connections" apiError={error}>
			{#snippet action()}
				<Button variant="ghost" class="underline" onclick={() => load(providerId)}>Try again</Button
				>
			{/snippet}
		</StateMessage>
	{:else if endpoints.length === 0}
		<StateMessage
			kind="empty"
			title="No connections yet"
			description="Connections hold the credential used to route. Add one to start."
		/>
	{:else}
		<EndpointTable
			{endpoints}
			selected={bulk.selected.ids}
			allSelected={bulk.selected.all}
			onopen={(entry) => (selected = entry)}
			ondelete={askDelete}
			onselect={(id, checked) => bulk.toggle(id, checked)}
		/>

		<BulkActionBar
			kind="connections"
			unit="connection"
			selected={bulk.count}
			{total}
			scope={bulkScope}
			allSelected={bulk.selected.all}
			ondisable={() => void runBulkStatus('disabled')}
			onenable={() => void runBulkStatus('active')}
			ondelete={() => void runBulkDelete()}
			ontag={(tag: string) => void runBulkTag(tag)}
			onclear={() => bulk.clear()}
			ontoggleall={(checked: boolean) => bulk.toggleEvery(checked)}
		/>

		{#if bulk.busy}
			<p role="status" class="text-sm text-[var(--color-text-muted)]">Applying to the set...</p>
		{:else if bulkNotice}
			<p role="status" class="text-sm text-[var(--color-text-muted)]">{bulkNotice}</p>
		{/if}
		{#if bulk.error}
			<p role="alert" class="text-sm text-[var(--color-danger)]">
				<ApiErrorText error={bulk.error} />
			</p>
		{/if}

		<PagingControls page={pageNumber} {lastPage} onpage={goPage} />
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
