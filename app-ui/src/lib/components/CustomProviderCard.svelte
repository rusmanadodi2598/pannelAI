<script lang="ts">
	// The custom provider card on a node's detail screen (docs/SPEC-UI/001-SPEC-UI.md §6.3).
	//
	// The provider read is not enough here, which is why this component does a second one: the gateway
	// synthesizes a provider entry from the node, and that entry carries the base URL and the wire format
	// but not the prefix, the api type, or when the node was created. Those are the facts an operator edits
	// against, so they are read from the node's own route (§7.4).
	//
	// The card is the reference's own (`providers/[id]/page.js`): its title names the node's type,
	// its first line is the request the gateway will make, and the three actions that change the node sit
	// on it. Add API Key belongs here as much as on the Connections section, because it is the first thing
	// an operator does with a node that has no connection yet. The prefix is reported upward because the
	// models section addresses a model as `prefix/model` and this is the read that holds it.
	//
	// Edit, Test, and Delete live here rather than in the list's rows, which is the reference's shape: the
	// list is for finding a node, and this is where its state changes. Delete confirms first because it cannot
	// be undone and it takes the node's connections, their keys and its model rows with it. The API still refuses
	// a node a combo names or an alias targets, and those answers are the only reason a confirmed delete comes
	// back unchanged.
	import { resolve } from '$app/paths';
	import { goto } from '$app/navigation';
	import { untrack } from 'svelte';
	import CustomProviderDialog from '$lib/components/CustomProviderDialog.svelte';
	import CustomProviderTest from '$lib/components/CustomProviderTest.svelte';
	import ProviderNodeDeleteDialog from '$lib/components/ProviderNodeDeleteDialog.svelte';
	import ProviderNodeFacts from '$lib/components/ProviderNodeFacts.svelte';
	import SectionFold from '$lib/components/SectionFold.svelte';
	import StateMessage from '$lib/components/StateMessage.svelte';
	import { Button } from '$lib/primitives/button/index.js';
	import type { ApiError } from '$lib/api/errors';
	import { CONTROL_ICONS, ROW_ACTION_ICONS } from '$lib/icons';
	import { listEndpointLabels } from '$lib/api/endpoints';
	import { deleteProviderNode, getProviderNode } from '$lib/api/provider-nodes';
	import {
		NODE_TYPE_LABELS,
		nodeEndpointLabel,
		nodeEndpointUrl,
		nodeTypeOfId,
		type NodeType,
		type ProviderNode
	} from '$lib/schemas/provider-node';

	// Add API Key keeps its label because it is the card's primary action; Edit and Delete are the two
	// verb actions beside it, so they follow the panel's icon-only row-action contract (SPEC-UI §8.11.9):
	// the glyph is decorative, and the button carries the name. Delete is the one that destroys the node,
	// which is why it alone wears the danger colour.
	const AddKeyIcon = CONTROL_ICONS.addKey.icon;
	const EditIcon = ROW_ACTION_ICONS.edit.icon;
	const DeleteIcon = ROW_ACTION_ICONS.delete.icon;

	let {
		providerId,
		onchanged,
		onaddkey,
		onprefix
	}: {
		providerId: string;
		/** Asks the page to re-read the provider, whose base URL an edit changes. */
		onchanged: () => void | Promise<void>;
		/** Opens the page's key dialog, which adds a connection to this node. */
		onaddkey?: () => void;
		/** Reports the node's model prefix, which the models section addresses its rows with. */
		onprefix?: (prefix: string) => void;
	} = $props();

	let node = $state<ProviderNode | null>(null);
	let loading = $state(true);
	let error = $state<ApiError | null>(null);

	let editing = $state(false);
	let confirming = $state(false);
	let deleting = $state(false);
	let deleteError = $state<ApiError | null>(null);
	/** The delete failure was the API's CONFLICT: a combo or an alias still points at this node. */
	let conflict = $state(false);
	// The connections a confirmed delete takes with it; `null` is the count not being known.
	let connections = $state<number | null>(null);

	// The type comes from the id, which is the public contract (§7.4), and not from the node: the dialog
	// needs it before the read lands, and a node whose read failed still has to open a usable dialog.
	const type = $derived<NodeType>(nodeTypeOfId(providerId) ?? 'openai-compatible');

	$effect(() => {
		const id = providerId;
		untrack(() => void load(id));
	});

	$effect(() => {
		if (node !== null) onprefix?.(node.prefix);
	});

	async function load(id: string): Promise<void> {
		loading = true;
		const result = await getProviderNode(id);

		if (!result.ok) {
			loading = false;
			error = result.error;
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

	// Read on opening rather than reused from the page's provider total, which no connection write below
	// refreshes: the dialog drops the number rather than state a stale one in front of a credential loss.
	async function askDelete(): Promise<void> {
		deleteError = null;
		conflict = false;
		connections = null;
		confirming = true;

		const result = await listEndpointLabels({ provider_id: providerId, page: 1, per_page: 1 });
		connections = result.ok ? result.data.meta.total : null;
	}

	async function remove(): Promise<void> {
		if (!node) return;

		deleting = true;
		const result = await deleteProviderNode(node.id);
		deleting = false;

		if (!result.ok) {
			conflict = result.error.code === 'CONFLICT';
			deleteError = result.error;
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
	<StateMessage kind="error" title="This custom provider could not be loaded" apiError={error}>
		{#snippet action()}
			<Button variant="ghost" class="underline" onclick={reload}>Try again</Button>
		{/snippet}
	</StateMessage>
{:else if node}
	<div
		class="flex flex-col gap-3 rounded-[var(--radius-md)] border border-[var(--color-border)] p-3"
	>
		<div class="flex flex-wrap items-start justify-between gap-3">
			<div class="flex flex-col gap-1">
				<h2 class="text-base font-medium">{NODE_TYPE_LABELS[type]} Details</h2>
				<p class="break-all text-sm text-[var(--color-text-muted)]">
					{nodeEndpointLabel(node)} · {nodeEndpointUrl(node)}
				</p>
			</div>
			<div class="flex flex-wrap gap-2">
				{#if onaddkey}
					<Button variant="outline" onclick={onaddkey}>
						<AddKeyIcon class="size-4" aria-hidden="true" />
						Add API Key
					</Button>
				{/if}
				<Button
					variant="ghost"
					size="icon"
					aria-label="Edit"
					title="Edit"
					onclick={() => {
						deleteError = null;
						conflict = false;
						editing = true;
					}}
				>
					<EditIcon class="size-4" aria-hidden="true" />
				</Button>
				<Button
					variant="destructive-ghost"
					size="icon"
					aria-label="Delete"
					title="Delete"
					onclick={askDelete}
				>
					<DeleteIcon class="size-4" aria-hidden="true" />
				</Button>
			</div>
		</div>

		<SectionFold title="Facts and test">
			<div class="flex flex-col gap-3">
				<ProviderNodeFacts {node} />
				<CustomProviderTest providerId={node.id} />
			</div>
		</SectionFold>
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

	<ProviderNodeDeleteDialog
		open={confirming}
		name={node.name}
		prefix={node.prefix}
		{connections}
		error={deleteError}
		{conflict}
		{deleting}
		onconfirm={remove}
		oncancel={() => (confirming = false)}
	/>
{/if}
