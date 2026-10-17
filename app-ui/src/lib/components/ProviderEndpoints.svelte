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
	import StateMessage from '$lib/components/StateMessage.svelte';
	import type { ApiError } from '$lib/api/errors';
	import { deleteEndpoint, listEndpoints } from '$lib/api/endpoints';
	import { bulkDelete, bulkStatus, bulkTags } from '$lib/api/bulk-mutation';
	import { CONTROL_ICONS } from '$lib/icons';
	import { emptySelection, toggleAll, toggleRow, type Selection } from '$lib/utils/selection';
	import type { Endpoint } from '$lib/schemas/endpoint';

	let { providerId, token = 0 }: { providerId: string; token?: number } = $props();

	const PreviousIcon = CONTROL_ICONS.previous.icon;
	const NextIcon = CONTROL_ICONS.next.icon;

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
	let chosen = $state<Selection>(emptySelection);
	let bulkBusy = $state(false);
	let bulkError = $state<ApiError | null>(null);
	let bulkNotice = $state<string | null>(null);

	const bulkCount = $derived(chosen.all ? total : chosen.ids.length);
	const bulkScope = $derived(
		chosen.all
			? `all ${total} connections at ${providerId}`
			: `${chosen.ids.length} of the ${endpoints.length} on this page`
	);

	function toggleChosen(id: string, checked: boolean): void {
		chosen = toggleRow(
			chosen,
			id,
			checked,
			endpoints.map((entry) => entry.id)
		);
	}

	function toggleChooseAll(checked: boolean): void {
		chosen = toggleAll(checked);
	}

	function bulkSelector(): { ids?: string[]; all?: true; provider_id?: string } {
		return chosen.all ? { all: true, provider_id: providerId } : { ids: chosen.ids };
	}

	async function runBulkStatus(status: 'active' | 'disabled'): Promise<void> {
		if (bulkCount === 0) return;
		bulkBusy = true;
		const result = await bulkStatus('endpoints', bulkSelector(), status, bulkCount);
		bulkBusy = false;
		if (!result.ok) {
			bulkError = result.error;
			return;
		}
		bulkError = null;
		bulkNotice = `${result.data.changed} connection(s) ${status === 'disabled' ? "parked; they return on their own when the provider's reset window runs out" : 'returned to rotation'}.`;
		await load(providerId);
	}

	async function runBulkDelete(): Promise<void> {
		if (bulkCount === 0) return;
		bulkBusy = true;
		const result = await bulkDelete('endpoints', bulkSelector(), bulkCount);
		bulkBusy = false;
		if (!result.ok) {
			bulkError = result.error;
			return;
		}
		bulkError = null;
		bulkNotice = `Removed ${result.data.changed} connection(s) and their stored keys.`;
		chosen = emptySelection;
		await load(providerId);
	}

	// The bar hands over a tag it already normalized; the server canonicalizes once more and refuses
	// a label too long to store rather than cutting it.
	async function runBulkTag(tag: string): Promise<void> {
		if (bulkCount === 0) return;
		bulkBusy = true;
		const result = await bulkTags('endpoints', bulkSelector(), [tag], [], false, bulkCount);
		bulkBusy = false;
		if (!result.ok) {
			bulkError = result.error;
			return;
		}
		bulkError = null;
		bulkNotice = `Tagged ${result.data.changed} connection(s) "${tag}".`;
		await load(providerId);
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
				<button
					type="button"
					class="inline-flex min-h-11 items-center underline"
					onclick={() => load(providerId)}>Try again</button
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
			selected={chosen.ids}
			allSelected={chosen.all}
			onopen={(entry) => (selected = entry)}
			ondelete={askDelete}
			onselect={toggleChosen}
		/>

		<BulkActionBar
			kind="connections"
			selected={bulkCount}
			{total}
			scope={bulkScope}
			allSelected={chosen.all}
			ondisable={() => void runBulkStatus('disabled')}
			onenable={() => void runBulkStatus('active')}
			ondelete={() => void runBulkDelete()}
			ontag={(tag: string) => void runBulkTag(tag)}
			onclear={() => (chosen = emptySelection)}
			ontoggleall={toggleChooseAll}
		/>

		{#if bulkBusy}
			<p role="status" class="mt-2 text-sm text-[var(--color-text-muted)]">
				Applying to the set...
			</p>
		{:else if bulkNotice}
			<p role="status" class="mt-2 text-sm text-[var(--color-text-muted)]">{bulkNotice}</p>
		{/if}
		{#if bulkError}
			<p role="alert" class="mt-2 text-sm text-[var(--color-danger)]">
				<ApiErrorText error={bulkError} />
			</p>
		{/if}

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
