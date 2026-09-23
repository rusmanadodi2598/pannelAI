<script lang="ts">
	// The rows of the custom-model list (docs/SPEC-UI/001-SPEC-UI.md §6.3).
	//
	// The section around this table owns the reads and the writes; the table renders what it was handed and
	// reports the one action a row has. It is its own component because a custom node's rows carry a fact a
	// registry provider's do not: the string the model is addressed by, `prefix/model`, which is what an
	// operator copies into a client. Without a prefix there is no such string, so the column is absent
	// rather than empty.
	import CopyButton from '$lib/components/CopyButton.svelte';
	import { customModelLabel, type CustomModel } from '$lib/schemas/custom-model';
	import { formatTimestamp } from '$lib/utils/time';

	let {
		rows,
		nodePrefix,
		removing,
		onremove
	}: {
		rows: CustomModel[];
		/** The node's model prefix, present only on a custom node's screen. */
		nodePrefix?: string;
		removing: boolean;
		onremove: (row: CustomModel) => void;
	} = $props();

	const prefix = $derived(nodePrefix !== undefined && nodePrefix !== '' ? nodePrefix : null);
</script>

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
			{#each rows as row (row.id)}
				<tr class="border-t border-[var(--color-border)]">
					<td class="px-3 py-2">
						<span class="font-medium">{customModelLabel(row)}</span>
						<br />
						<span class="text-[var(--color-text-muted)]">{row.model_id}</span>
						{#if prefix !== null}
							<span class="mt-1 flex flex-wrap items-center gap-2">
								<code class="break-all text-xs">{prefix}/{row.model_id}</code>
								<CopyButton value={`${prefix}/${row.model_id}`} />
							</span>
						{/if}
					</td>
					<td class="px-3 py-2">
						{#if row.capabilities.length === 0}
							<span class="text-[var(--color-text-muted)]">None declared</span>
						{:else}
							<span class="flex flex-wrap gap-1">
								{#each row.capabilities as entry (entry)}
									<span class="rounded-[var(--radius-sm)] bg-[var(--color-surface-2)] px-2 py-0.5"
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
							onclick={() => onremove(row)}>Remove</button
						>
					</td>
				</tr>
			{/each}
		</tbody>
	</table>
</div>
