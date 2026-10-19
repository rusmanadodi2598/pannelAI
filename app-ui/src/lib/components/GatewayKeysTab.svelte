<script lang="ts">
	// Gateway keys table of Endpoint & Key (docs/SPEC-UI/001-SPEC-UI.md §6.2).
	//
	// This owns the list, its paging, and the confirmation dialog for the destructive action. Creating,
	// rendering a row, and showing the one-time key each live in their own component, which keeps every
	// file under the project line limit.
	//
	// Revoked rows never reach the table: DELETE is terminal (SPEC-API §7.3), so a key carrying that
	// status has no control left that would work, and the reference's own list drops a deleted key. The
	// route still counts revoked rows in `meta.total`, which is what the paging math reads; the
	// route-side filter is requested in docs/PORT/001-PORT-ENDPOINT-KEYS.md F1.
	import CreateGatewayKeyForm from '$lib/components/CreateGatewayKeyForm.svelte';
	import GatewayKeyRow from '$lib/components/GatewayKeyRow.svelte';
	import Modal from '$lib/components/Modal.svelte';
	import OneTimeKeyModal from '$lib/components/OneTimeKeyModal.svelte';
	import RefreshControl from '$lib/components/RefreshControl.svelte';
	import PagingControls from '$lib/components/PagingControls.svelte';
	import StateMessage from '$lib/components/StateMessage.svelte';
	import ApiErrorText from '$lib/components/ApiErrorText.svelte';
	import { Button } from '$lib/primitives/button/index.js';
	import type { ApiError } from '$lib/api/errors';
	import { listGatewayKeys, revokeGatewayKey } from '$lib/api/gateway-keys';
	import {
		KEY_STATUS_REVOKED,
		type CreatedGatewayKey,
		type GatewayKey
	} from '$lib/schemas/gateway-key';
	import { onMount } from 'svelte';

	const PAGE_SIZE = 25;

	let keys = $state<GatewayKey[]>([]);
	let total = $state(0);
	let page = $state(1);
	let loading = $state(true);
	let error = $state<ApiError | null>(null);

	let created = $state<CreatedGatewayKey | null>(null);
	let pendingDelete = $state<GatewayKey | null>(null);
	let deleting = $state(false);
	// A failed delete is not a failed list read, so it gets its own line rather than replacing the table
	// with an error state.
	let notice = $state<ApiError | null>(null);

	const lastPage = $derived(Math.max(1, Math.ceil(total / PAGE_SIZE)));

	onMount(load);

	async function load(): Promise<void> {
		loading = true;
		const result = await listGatewayKeys({ page, per_page: PAGE_SIZE });
		loading = false;

		if (!result.ok) {
			error = result.error;
			return;
		}

		error = null;
		// The terminal rows stay in the server's count, which the paging math reads, but they do not
		// render: a revoked key has no working action left, so it leaves the screen.
		keys = result.data.data.filter((entry) => entry.status !== KEY_STATUS_REVOKED);
		total = result.data.meta.total;
	}

	function goPage(next: number): void {
		page = next;
		void load();
	}

	async function confirmDelete(): Promise<void> {
		if (!pendingDelete) return;
		const target = pendingDelete;
		pendingDelete = null;
		deleting = true;
		const result = await revokeGatewayKey(target.id);
		deleting = false;

		if (!result.ok) {
			notice = result.error;
			return;
		}

		notice = null;
		await load();
	}
</script>

<div class="flex flex-col gap-5">
	<CreateGatewayKeyForm
		oncreated={(result) => {
			created = result;
			void load();
		}}
	/>

	<!-- The same mobile width as the create form above: full width on a phone, so Refresh lines up with
	     Create gateway key instead of sitting at its own size. RefreshControl is shared, so the rule lives here. -->
	<div
		class="flex flex-col sm:flex-row [&>div]:w-full sm:[&>div]:w-auto [&_button]:w-full sm:[&_button]:w-auto [&_button]:justify-center"
	>
		<RefreshControl onrefresh={load} />
	</div>

	{#if notice}
		<p role="alert" class="text-sm text-[var(--color-danger)]">
			<ApiErrorText error={notice} />
		</p>
	{/if}

	{#if loading}
		<StateMessage kind="loading" title="Loading gateway keys" />
	{:else if error}
		<StateMessage kind="error" title="Gateway keys could not be loaded" apiError={error}>
			{#snippet action()}
				<Button variant="ghost" class="underline" onclick={load}>Try again</Button>
			{/snippet}
		</StateMessage>
	{:else if total === 0}
		<StateMessage
			kind="empty"
			title="No gateway keys yet"
			description="Create one to let a client reach the gateway."
		/>
	{:else}
		{#if keys.length === 0}
			<StateMessage
				kind="empty"
				title="No keys on this page"
				description="Every key on this page has been deleted."
			/>
		{:else}
			<div
				class="mobile-key-value-frame overflow-x-auto rounded-[var(--radius-md)] border border-[var(--color-border)]"
			>
				<table class="mobile-key-value-table w-full border-collapse text-sm">
					<caption class="sr-only">Gateway keys</caption>
					<thead class="bg-[var(--color-surface-2)] text-left">
						<tr>
							<th scope="col" class="px-3 py-2 font-medium">Name</th>
							<th scope="col" class="px-3 py-2 font-medium">Key</th>
							<th scope="col" class="px-3 py-2 font-medium">Status</th>
							<th scope="col" class="px-3 py-2 font-medium">Requests</th>
							<th scope="col" class="px-3 py-2 font-medium">Last used</th>
							<th scope="col" class="px-3 py-2 font-medium">Actions</th>
						</tr>
					</thead>
					<tbody>
						{#each keys as key (key.id)}
							<GatewayKeyRow
								entry={key}
								busy={deleting}
								ondelete={(target) => (pendingDelete = target)}
								onchanged={load}
							/>
						{/each}
					</tbody>
				</table>
			</div>
		{/if}

		<PagingControls {page} {lastPage} onpage={goPage} />
	{/if}
</div>

<OneTimeKeyModal {created} onclose={() => (created = null)} />

<Modal
	title="Delete this gateway key"
	open={pendingDelete !== null}
	onclose={() => (pendingDelete = null)}
>
	{#if pendingDelete}
		<p class="text-sm">
			Deleting <span class="font-medium">{pendingDelete.name}</span> ({pendingDelete.key_hint})
			stops its clients. It cannot be undone.
		</p>
	{/if}

	{#snippet footer()}
		<Button variant="outline" onclick={() => (pendingDelete = null)}>Keep the key</Button>
		<Button variant="destructive" aria-busy={deleting} onclick={confirmDelete}>Delete key</Button>
	{/snippet}
</Modal>
