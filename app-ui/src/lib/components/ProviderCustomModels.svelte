<script lang="ts">
	// The custom models declared for this provider (docs/SPEC-UI/001-SPEC-UI.md §6.3).
	//
	// The list route reads every provider's rows, so the panel narrows for display, the same way the
	// catalog narrows by provider. Each write is one row at a time, not a set: the API has a create and a
	// delete route and no replace, so there is no set to merge over and no partial-write hazard here.
	//
	// The order is the route's own, `created_at DESC, id DESC`, which is why an added row is prepended
	// rather than re-read: the POST answer is the row the server stored.
	//
	// A custom node passes its prefix, and that changes what the section is. A registry provider's custom
	// rows sit beside a catalog the registry already ships, so the rows are a supplement and the section
	// says so. A node has no registry catalog at all: these rows ARE its models, which is why the section
	// then states the string each one is addressed by, offers the import the reference offers, and words
	// its empty state for the node rather than for a supplement.
	import { untrack } from 'svelte';
	import CustomModelDeleteDialog from '$lib/components/CustomModelDeleteDialog.svelte';
	import CustomModelForm from '$lib/components/CustomModelForm.svelte';
	import CustomModelTable from '$lib/components/CustomModelTable.svelte';
	import NodeModelsImport from '$lib/components/NodeModelsImport.svelte';
	import StateMessage from '$lib/components/StateMessage.svelte';
	import { deleteCustomModel, listCustomModels } from '$lib/api/models';
	import { providerCustomModels, type CustomModel } from '$lib/schemas/custom-model';
	import { nodeTypeOfId } from '$lib/schemas/provider-node';
	import type { ProviderThinkingStore } from '$lib/stores/provider-thinking.svelte';

	let {
		providerId,
		thinking,
		onchanged,
		nodePrefix
	}: {
		providerId: string;
		thinking: ProviderThinkingStore;
		onchanged: () => void;
		/** The node's model prefix, present only on a custom node's screen. */
		nodePrefix?: string;
	} = $props();

	// A node's models are addressed as `prefix/model`, so the prefix is what makes the model string this
	// section shows the one a client can send. Without it the rows are a registry provider's supplement.
	const prefix = $derived(nodePrefix !== undefined && nodePrefix !== '' ? nodePrefix : null);
	const vendor = $derived(
		nodeTypeOfId(providerId) === 'anthropic-compatible' ? 'Anthropic' : 'OpenAI'
	);

	let rows = $state<CustomModel[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let notice = $state<string | null>(null);

	// The row awaiting confirmation, and the state of that removal.
	let pending = $state<CustomModel | null>(null);
	let removing = $state(false);
	let removeError = $state<string | null>(null);

	const mine = $derived(providerCustomModels(rows, providerId));

	// The list is every provider's rows, so it is read once per mount: a route change to another provider
	// narrows the same rows and needs no second request.
	$effect(() => {
		untrack(() => void load());
	});

	async function load(): Promise<void> {
		loading = true;
		const result = await listCustomModels();
		loading = false;

		if (!result.ok) {
			error = result.error.message;
			return;
		}

		error = null;
		rows = result.data.data;
	}

	function added(row: CustomModel): void {
		rows = [row, ...rows];
		notice = `${row.provider_id}/${row.model_id} was added to the catalog.`;
		onchanged();
	}

	// The import writes several rows before it answers, so it hands them over in one batch. The batch is
	// reversed because the route orders by `created_at DESC`: the last row written is the newest, and
	// prepending the batch in arrival order would put the oldest of them first. The import reports its own
	// outcome beside its own control, so this does not repeat it here.
	function imported(batch: CustomModel[]): void {
		rows = [...batch.slice().reverse(), ...rows];
		onchanged();
	}

	async function confirmRemove(): Promise<void> {
		if (!pending) return;

		removing = true;
		const result = await deleteCustomModel(pending.id);
		removing = false;

		if (!result.ok) {
			removeError = result.error.message;
			return;
		}

		const removed = pending;
		rows = rows.filter((row) => row.id !== removed.id);
		pending = null;
		removeError = null;
		notice = `${removed.provider_id}/${removed.model_id} is no longer declared.`;
		onchanged();
	}
</script>

<div class="flex flex-col gap-4">
	{#if prefix !== null}
		<p class="text-sm text-[var(--color-text-muted)]">
			Add {vendor}-compatible models manually or import them from the node's /models endpoint.
		</p>
	{/if}

	{#if loading}
		<StateMessage kind="loading" title="Loading the custom models" />
	{:else if error}
		<StateMessage kind="error" title="The custom models could not be loaded" description={error}>
			{#snippet action()}
				<button type="button" class="underline" onclick={() => load()}>Try again</button>
			{/snippet}
		</StateMessage>
	{:else if mine.length === 0}
		<StateMessage
			kind="empty"
			title={prefix === null ? 'No custom models for this provider' : 'No models for this node yet'}
			description={prefix === null
				? "The catalog for this provider is the registry's own list. A model added below joins it."
				: `A model declared here is addressed as ${prefix}/model-id. Add one below, or import the list the node's own /models endpoint answers.`}
		/>
	{:else}
		<CustomModelTable
			rows={mine}
			{thinking}
			{nodePrefix}
			{removing}
			onremove={(row) => {
				pending = row;
				removeError = null;
			}}
		/>

		<p class="text-sm text-[var(--color-text-muted)]">
			{mine.length}
			{mine.length === 1 ? 'custom model' : 'custom models'} for this provider. Another provider's rows
			are not listed here.
		</p>
	{/if}

	<!-- The form waits for the list: a row added while the list is in an error state would join rows the
	     panel never read, and the operator would see the add succeed with nothing to show for it. -->
	{#if !loading && !error}
		<CustomModelForm {providerId} onadded={added} />

		{#if prefix !== null}
			<NodeModelsImport
				{providerId}
				known={mine.map((row) => row.model_id)}
				onimported={imported}
			/>
		{/if}

		{#if notice}
			<p class="text-sm" role="status">{notice}</p>
		{/if}
	{/if}
</div>

<CustomModelDeleteDialog
	model={pending}
	error={removeError}
	{removing}
	onconfirm={() => void confirmRemove()}
	oncancel={() => {
		pending = null;
		removeError = null;
	}}
/>
