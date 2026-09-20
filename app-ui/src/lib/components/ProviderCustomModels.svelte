<script lang="ts">
	// The custom models declared for this provider (docs/SPEC-UI/001-SPEC-UI.md §6.3).
	//
	// The list route reads every provider's rows, so the panel narrows for display, the same way the
	// catalog narrows by provider. Each write is one row at a time, not a set: the API has a create and a
	// delete route and no replace, so there is no set to merge over and no partial-write hazard here.
	//
	// The order is the route's own, `created_at DESC, id DESC`, which is why an added row is prepended
	// rather than re-read: the POST answer is the row the server stored.
	import { untrack } from 'svelte';
	import CustomModelDeleteDialog from '$lib/components/CustomModelDeleteDialog.svelte';
	import CustomModelForm from '$lib/components/CustomModelForm.svelte';
	import StateMessage from '$lib/components/StateMessage.svelte';
	import { deleteCustomModel, listCustomModels } from '$lib/api/models';
	import {
		customModelLabel,
		providerCustomModels,
		type CustomModel
	} from '$lib/schemas/custom-model';
	import { formatTimestamp } from '$lib/utils/time';

	let { providerId, onchanged }: { providerId: string; onchanged: () => void } = $props();

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
			title="No custom models for this provider"
			description="The catalog for this provider is the registry's own list. A model added below joins it."
		/>
	{:else}
		<div class="overflow-x-auto rounded-[var(--radius-md)] border border-[var(--color-border)]">
			<table class="w-full min-w-[44rem] border-collapse text-sm">
				<caption class="sr-only">Custom models declared for this provider</caption>
				<thead class="bg-[var(--color-surface-2)] text-left">
					<tr>
						<th scope="col" class="px-3 py-2 font-medium">Model</th>
						<th scope="col" class="px-3 py-2 font-medium">Capabilities</th>
						<th scope="col" class="px-3 py-2 font-medium">Added</th>
						<th scope="col" class="px-3 py-2 font-medium">
							<span class="sr-only">Actions</span>
						</th>
					</tr>
				</thead>
				<tbody>
					{#each mine as row (row.id)}
						<tr class="border-t border-[var(--color-border)]">
							<td class="px-3 py-2">
								<span class="font-medium">{customModelLabel(row)}</span>
								<br />
								<span class="text-[var(--color-text-muted)]">{row.model_id}</span>
							</td>
							<td class="px-3 py-2">
								{#if row.capabilities.length === 0}
									<span class="text-[var(--color-text-muted)]">None declared</span>
								{:else}
									<span class="flex flex-wrap gap-1">
										{#each row.capabilities as entry (entry)}
											<span
												class="rounded-[var(--radius-sm)] bg-[var(--color-surface-2)] px-2 py-0.5"
												>{entry}</span
											>
										{/each}
									</span>
								{/if}
							</td>
							<td class="px-3 py-2">{formatTimestamp(row.created_at)}</td>
							<td class="px-3 py-2 text-end">
								<button
									type="button"
									class="min-h-11 underline disabled:opacity-50"
									disabled={removing}
									onclick={() => {
										pending = row;
										removeError = null;
									}}>Remove</button
								>
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>

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
