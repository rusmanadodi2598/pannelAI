<script lang="ts">
	// The custom provider card on a node's detail screen (docs/SPEC-UI/001-SPEC-UI.md §6.3).
	//
	// The provider read is not enough here, which is why this component does a second one: the gateway
	// synthesizes a provider entry from the node, and that entry carries the base URL and the wire format
	// but not the prefix, the api type, or when the node was created. Those are the facts an operator edits
	// against, so they are read from the node's own route (§7.4).
	//
	// Edit, Test, and Delete live here rather than in the list's rows, which is the reference's shape: the
	// list is for finding a node, and this is where its state changes. Delete confirms first because it
	// cannot be undone, and the API refuses it outright while an endpoint still references the node.
	import { resolve } from '$app/paths';
	import { goto } from '$app/navigation';
	import { untrack } from 'svelte';
	import CustomProviderDialog from '$lib/components/CustomProviderDialog.svelte';
	import CustomProviderTest from '$lib/components/CustomProviderTest.svelte';
	import Modal from '$lib/components/Modal.svelte';
	import StateMessage from '$lib/components/StateMessage.svelte';
	import { deleteProviderNode, getProviderNode } from '$lib/api/provider-nodes';
	import {
		nodeEndpointLabel,
		nodeEndpointPath,
		nodeEndpointUrl,
		nodeTypeOfId,
		type NodeType,
		type ProviderNode
	} from '$lib/schemas/provider-node';
	import { formatTimestamp } from '$lib/utils/time';

	let {
		providerId,
		onchanged
	}: {
		providerId: string;
		/** Asks the page to re-read the provider, whose base URL an edit changes. */
		onchanged: () => void | Promise<void>;
	} = $props();

	let node = $state<ProviderNode | null>(null);
	let loading = $state(true);
	let error = $state<string | null>(null);

	let editing = $state(false);
	let confirming = $state(false);
	let deleting = $state(false);
	let deleteError = $state<string | null>(null);
	/** The delete failure was the API's CONFLICT: an endpoint still references this node. */
	let conflict = $state(false);

	// The type comes from the id, which is the public contract (§7.4), and not from the node: the dialog
	// needs it before the read lands, and a node whose read failed still has to open a usable dialog.
	const type = $derived<NodeType>(nodeTypeOfId(providerId) ?? 'openai-compatible');

	$effect(() => {
		const id = providerId;
		untrack(() => void load(id));
	});

	async function load(id: string): Promise<void> {
		loading = true;
		const result = await getProviderNode(id);

		if (!result.ok) {
			loading = false;
			error = result.error.message;
			node = null;
			return;
		}

		loading = false;
		error = null;
		node = result.data;
	}

	async function reload(): Promise<void> {
		await load(providerId);
	}

	async function remove(): Promise<void> {
		if (!node) return;

		deleting = true;
		const result = await deleteProviderNode(node.id);
		deleting = false;

		if (!result.ok) {
			conflict = result.error.code === 'CONFLICT';
			deleteError = result.error.message;
			return;
		}

		deleteError = null;
		conflict = false;
		confirming = false;
		await goto(resolve('/providers'));
	}
</script>

{#if loading}
	<StateMessage kind="loading" title="Loading the custom provider" />
{:else if error}
	<StateMessage kind="error" title="This custom provider could not be loaded" description={error}>
		{#snippet action()}
			<button type="button" class="underline" onclick={reload}>Try again</button>
		{/snippet}
	</StateMessage>
{:else if node}
	<div
		class="flex flex-col gap-3 rounded-[var(--radius-md)] border border-[var(--color-border)] p-3"
	>
		<div class="flex flex-wrap items-start justify-between gap-3">
			<div class="flex flex-col gap-1">
				<h2 class="text-base font-medium">Custom provider</h2>
				<p class="text-sm text-[var(--color-text-muted)]">
					The gateway appends {nodeEndpointPath(node)} to this base URL, so it calls
					<span class="break-all">{nodeEndpointUrl(node)}</span>.
				</p>
			</div>
			<div class="flex flex-wrap gap-2">
				<button
					type="button"
					class="min-h-11 rounded-[var(--radius-sm)] border border-[var(--color-border)] px-3 text-sm"
					onclick={() => {
						deleteError = null;
						conflict = false;
						editing = true;
					}}>Edit</button
				>
				<button
					type="button"
					class="min-h-11 rounded-[var(--radius-sm)] border border-[var(--color-border)] px-3 text-sm"
					onclick={() => {
						deleteError = null;
						conflict = false;
						confirming = true;
					}}>Delete</button
				>
			</div>
		</div>

		<dl class="grid gap-x-6 gap-y-3 sm:grid-cols-2 lg:grid-cols-3">
			<div class="flex flex-col gap-0.5">
				<dt class="text-xs text-[var(--color-text-muted)]">Model prefix</dt>
				<dd class="text-sm">
					{node.prefix}/model
				</dd>
			</div>
			<div class="flex flex-col gap-0.5">
				<dt class="text-xs text-[var(--color-text-muted)]">Base URL</dt>
				<dd class="break-all text-sm">{node.base_url}</dd>
			</div>
			<div class="flex flex-col gap-0.5">
				<dt class="text-xs text-[var(--color-text-muted)]">Endpoint</dt>
				<dd class="text-sm">{nodeEndpointLabel(node)}</dd>
			</div>
			<div class="flex flex-col gap-0.5">
				<dt class="text-xs text-[var(--color-text-muted)]">Wire format</dt>
				<dd class="text-sm">{node.format}</dd>
			</div>
			<div class="flex flex-col gap-0.5">
				<dt class="text-xs text-[var(--color-text-muted)]">Created</dt>
				<dd class="text-sm">{formatTimestamp(node.created_at)}</dd>
			</div>
			<div class="flex flex-col gap-0.5">
				<dt class="text-xs text-[var(--color-text-muted)]">Last changed</dt>
				<dd class="text-sm">{formatTimestamp(node.updated_at)}</dd>
			</div>
		</dl>

		<CustomProviderTest providerId={node.id} />
	</div>

	<CustomProviderDialog
		target={editing ? node : null}
		{type}
		onsaved={async () => {
			await reload();
			await onchanged();
		}}
		onclose={() => (editing = false)}
	/>

	<Modal title="Delete this custom provider" open={confirming} onclose={() => (confirming = false)}>
		<p>
			Delete <span class="font-medium">{node.name}</span> ({node.prefix}/model)? Requests naming a
			model under that prefix stop resolving.
		</p>

		{#if deleteError}
			<p class="mt-3 text-[var(--color-danger)]" role="alert">
				{#if conflict}
					This provider is still referenced, so it was not deleted. {deleteError} Remove or move its endpoints
					first.
				{:else}
					This provider was not deleted. {deleteError}
				{/if}
			</p>
		{/if}

		{#snippet footer()}
			<button type="button" class="min-h-11 underline" onclick={() => (confirming = false)}
				>Keep it</button
			>
			<button
				type="button"
				class="min-h-11 rounded-[var(--radius-sm)] bg-[var(--color-danger)] px-4 text-[var(--color-accent-text)] disabled:opacity-50"
				disabled={deleting}
				onclick={remove}
			>
				{deleting ? 'Deleting' : 'Delete the provider'}
			</button>
		{/snippet}
	</Modal>
{/if}
