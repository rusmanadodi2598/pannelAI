<script lang="ts">
	// The models this provider cannot route (docs/SPEC-UI/001-SPEC-UI.md §6.3).
	//
	// The merged catalog excludes a disabled model, so this list is the only place one is visible and the
	// only place it can be turned back on. The heading and the count both say "for this provider" because
	// the route reads every provider's rows: the panel narrows for display, and the store keeps the whole
	// set for the write.
	//
	// A row shows the model id rather than a display name. The catalog cannot name a row it hides, and the
	// id is what the API stores and keys on, so it is the honest thing to show.
	import StateMessage from '$lib/components/StateMessage.svelte';
	import type { ModelDisabledStore } from '$lib/stores/model-disabled.svelte';
	import { disabledRefKey, type DisabledRef } from '$lib/schemas/model-disabled';

	let {
		providerId,
		disabled,
		onchanged
	}: {
		providerId: string;
		disabled: ModelDisabledStore;
		onchanged: () => void;
	} = $props();

	const mine = $derived(disabled.mine(providerId));
	const mineKeys = $derived(new Set(mine.map((ref) => disabledRefKey(ref))));

	// The last write's outcome renders in whichever half of the screen holds the ref it names: a row that
	// was just disabled is here, and one that was turned back on is not.
	const outcome = $derived(
		disabled.outcome && mineKeys.has(disabled.outcome.key) ? disabled.outcome : null
	);

	async function enable(ref: DisabledRef): Promise<void> {
		if (await disabled.setDisabled(ref, false)) onchanged();
	}
</script>

<div class="flex flex-col gap-3">
	{#if disabled.loading}
		<StateMessage kind="loading" title="Loading the disabled models" />
	{:else if disabled.error}
		<StateMessage
			kind="error"
			title="The disabled models could not be loaded"
			description={disabled.error}
		>
			{#snippet action()}
				<button type="button" class="underline" onclick={() => disabled.load()}>Try again</button>
			{/snippet}
		</StateMessage>
	{:else if mine.length === 0}
		<StateMessage
			kind="empty"
			title="No models are disabled for this provider"
			description="Every model in the catalog can be routed. A model disabled from that list leaves it until it is enabled here."
		/>
	{:else}
		<!-- relative keeps the sr-only column label (position: absolute) inside this scroll box. -->
		<div
			class="relative overflow-x-auto rounded-[var(--radius-md)] border border-[var(--color-border)]"
		>
			<table class="w-full min-w-[28rem] border-collapse text-sm">
				<caption class="sr-only">Models disabled for this provider</caption>
				<thead class="bg-[var(--color-surface-2)] text-left">
					<tr>
						<th scope="col" class="px-3 py-2 font-medium">Model</th>
						<th scope="col" class="px-3 py-2 font-medium">
							<span class="sr-only">Actions</span>
						</th>
					</tr>
				</thead>
				<tbody>
					{#each mine as ref (disabledRefKey(ref))}
						<tr class="border-t border-[var(--color-border)]">
							<td class="px-3 py-2">
								<span class="font-medium">{ref.model_id}</span>
								<br />
								<span class="text-[var(--color-text-muted)]">Not routable</span>
							</td>
							<td class="px-3 py-2 text-end">
								<button
									type="button"
									class="min-h-11 underline disabled:opacity-50"
									disabled={disabled.saving !== null}
									onclick={() => enable(ref)}
								>
									{disabled.saving === disabledRefKey(ref) ? 'Enabling' : 'Enable'}
								</button>
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>

		<p class="text-sm text-[var(--color-text-muted)]">
			{mine.length}
			{mine.length === 1 ? 'model is' : 'models are'} disabled for this provider. Another provider's disabled
			models are not listed here.
		</p>
	{/if}

	{#if outcome}
		<p
			class="text-sm"
			class:text-[var(--color-danger)]={!outcome.ok}
			role={outcome.ok ? 'status' : 'alert'}
		>
			{outcome.message}
		</p>
	{/if}
</div>
