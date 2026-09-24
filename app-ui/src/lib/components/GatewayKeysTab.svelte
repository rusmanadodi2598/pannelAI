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
	import StateMessage from '$lib/components/StateMessage.svelte';
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
	let error = $state<string | null>(null);

	let created = $state<CreatedGatewayKey | null>(null);
	let pendingDelete = $state<GatewayKey | null>(null);
	let deleting = $state(false);
	// A failed delete is not a failed list read, so it gets its own line rather than replacing the table
	// with an error state.
	let notice = $state<string | null>(null);

	const lastPage = $derived(Math.max(1, Math.ceil(total / PAGE_SIZE)));

	onMount(load);

	async function load(): Promise<void> {
		loading = true;
		const result = await listGatewayKeys({ page, per_page: PAGE_SIZE });
		loading = false;

		if (!result.ok) {
			error = result.error.message;
			return;
		}

		error = null;
		// The terminal rows stay in the server's count, which the paging math reads, but they do not
		// render: a revoked key has no working action left, so it leaves the screen.
		keys = result.data.data.filter((entry) => entry.status !== KEY_STATUS_REVOKED);
		total = result.data.meta.total;
	}

	async function confirmDelete(): Promise<void> {
		if (!pendingDelete) return;
		const target = pendingDelete;
		pendingDelete = null;
		deleting = true;
		const result = await revokeGatewayKey(target.id);
		deleting = false;

		if (!result.ok) {
			notice = result.error.message;
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

	<RefreshControl onrefresh={load} />

	{#if notice}
		<p role="alert" class="text-sm text-[var(--color-danger)]">{notice}</p>
	{/if}

	{#if loading}
		<StateMessage kind="loading" title="Loading gateway keys" />
	{:else if error}
		<StateMessage kind="error" title="Gateway keys could not be loaded" description={error}>
			{#snippet action()}
				<button type="button" class="underline" onclick={load}>Try again</button>
			{/snippet}
		</StateMessage>
	{:else if total === 0}
		<StateMessage
			kind="empty"
			title="No gateway keys yet"
			description="Create one to let a CLI tool reach the gateway."
		/>
	{:else}
		{#if keys.length === 0}
			<StateMessage
				kind="empty"
				title="No keys on this page"
				description="Every key on this page has been deleted."
			/>
		{:else}
			<div class="overflow-x-auto rounded-[var(--radius-md)] border border-[var(--color-border)]">
				<table class="w-full min-w-[44rem] border-collapse text-sm">
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
			stops every client using it immediately. The key cannot be restored.
		</p>
	{/if}

	{#snippet footer()}
		<button
			type="button"
			class="min-h-11 rounded-[var(--radius-sm)] border border-[var(--color-border)] px-3 text-sm"
			onclick={() => (pendingDelete = null)}>Keep the key</button
		>
		<button
			type="button"
			class="min-h-11 rounded-[var(--radius-sm)] bg-[var(--color-danger)] px-3 text-sm font-medium text-white"
			onclick={confirmDelete}>Delete key</button
		>
	{/snippet}
</Modal>
