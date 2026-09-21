<script lang="ts">
	// Gateway keys tab of Endpoint & Key (docs/SPEC-UI/001-SPEC-UI.md §6.2, tab 1).
	//
	// This owns the list, its paging, and the two confirmation dialogs. Creating, rendering a row, and
	// showing the one-time key each live in their own component, which keeps every file under the project
	// line limit. The upstream endpoints tab is its sibling and the page host swaps between them.
	import CreateGatewayKeyForm from '$lib/components/CreateGatewayKeyForm.svelte';
	import GatewayKeyRow from '$lib/components/GatewayKeyRow.svelte';
	import Modal from '$lib/components/Modal.svelte';
	import OneTimeKeyModal from '$lib/components/OneTimeKeyModal.svelte';
	import StateMessage from '$lib/components/StateMessage.svelte';
	import { listGatewayKeys, revokeGatewayKey } from '$lib/api/gateway-keys';
	import type { CreatedGatewayKey, GatewayKey } from '$lib/schemas/gateway-key';
	import { onMount } from 'svelte';

	const PAGE_SIZE = 25;

	let keys = $state<GatewayKey[]>([]);
	let total = $state(0);
	let page = $state(1);
	let loading = $state(true);
	let error = $state<string | null>(null);

	let created = $state<CreatedGatewayKey | null>(null);
	let pendingRevoke = $state<GatewayKey | null>(null);
	let revoking = $state(false);
	// A failed revoke is not a failed list read, so it gets its own line rather than replacing the table
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
		keys = result.data.data;
		total = result.data.meta.total;
	}

	async function confirmRevoke(): Promise<void> {
		if (!pendingRevoke) return;
		const target = pendingRevoke;
		pendingRevoke = null;
		revoking = true;
		const result = await revokeGatewayKey(target.id);
		revoking = false;

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
	{:else if keys.length === 0}
		<StateMessage
			kind="empty"
			title="No gateway keys yet"
			description="Create one to let a CLI tool reach the gateway."
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
							busy={revoking}
							onrevoke={(target) => (pendingRevoke = target)}
							onchanged={load}
						/>
					{/each}
				</tbody>
			</table>
		</div>

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
	title="Revoke this gateway key"
	open={pendingRevoke !== null}
	onclose={() => (pendingRevoke = null)}
>
	{#if pendingRevoke}
		<p class="text-sm">
			Revoking <span class="font-medium">{pendingRevoke.name}</span> ({pendingRevoke.key_hint})
			stops every client using it immediately.
		</p>
	{/if}

	{#snippet footer()}
		<button
			type="button"
			class="min-h-11 rounded-[var(--radius-sm)] border border-[var(--color-border)] px-3 text-sm"
			onclick={() => (pendingRevoke = null)}>Keep the key</button
		>
		<button
			type="button"
			class="min-h-11 rounded-[var(--radius-sm)] bg-[var(--color-danger)] px-3 text-sm font-medium text-white"
			onclick={confirmRevoke}>Revoke key</button
		>
	{/snippet}
</Modal>
