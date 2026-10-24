<script lang="ts">
	// The upstream endpoint table (docs/SPEC-UI/001-SPEC-UI.md §6.2, tab 2).
	//
	// Columns follow the decision the operator makes here: which connection is failing and where it sits in
	// the routing order. Provider and endpoint come first to identify the row, priority and status decide
	// whether it needs attention, and the key and test columns say what the router last saw. Detail is a
	// button rather than a clickable row, so the action is reachable by keyboard and announced as a control.
	//
	// Delete is a per-row icon-only action that appears only when the caller handles it: this table is the
	// endpoint list of §6.2 and the provider's connection list of §6.3, and only the latter wires a removal.
	// The confirmation lives with the caller, so this only asks.
	//
	// Selection is offered the same way: when the caller can act on a set, each row's Actions cell gains a
	// checkbox. Select-All is deliberately not here: `src/app.css` clips the whole `<thead>` below 768px,
	// so a header control would not exist on a phone. `BulkActionBar` carries it instead.
	import { Button } from '$lib/primitives/button/index.js';
	import { AUTH_TYPE_LABELS, ENDPOINT_STATUS_ACTIVE, type Endpoint } from '$lib/schemas/endpoint';
	import { ROW_ACTION_ICONS } from '$lib/icons';
	import { rowChecked } from '$lib/utils/selection';

	let {
		endpoints,
		onopen,
		ondelete,
		selected = [],
		allSelected = false,
		onselect
	}: {
		endpoints: Endpoint[];
		onopen: (entry: Endpoint) => void;
		/** Present when this list can remove a connection; renders the Delete button. */
		ondelete?: (entry: Endpoint) => void;
		/** Ids the caller has selected. Present alongside `onselect` to render the checkbox. */
		selected?: string[];
		/** Select-All is on, which contains every row this page loaded. */
		allSelected?: boolean;
		onselect?: (id: string, checked: boolean) => void;
	} = $props();

	const DeleteIcon = ROW_ACTION_ICONS.delete.icon;

	const selectable = $derived(onselect !== undefined);

	function isChecked(id: string): boolean {
		return rowChecked({ ids: selected, all: allSelected }, id);
	}

	function statusLabel(status: string): string {
		return status === ENDPOINT_STATUS_ACTIVE ? 'Active' : status;
	}
</script>

<div
	class="mobile-key-value-frame overflow-x-auto rounded-[var(--radius-md)] border border-[var(--color-border)]"
>
	<table class="mobile-key-value-table w-full border-collapse text-sm">
		<caption class="sr-only">Upstream endpoints, ordered by routing priority</caption>
		<thead class="bg-[var(--color-surface-2)] text-left">
			<tr>
				<th scope="col" class="px-3 py-2 font-medium">Provider</th>
				<th scope="col" class="px-3 py-2 font-medium">Endpoint</th>
				<th scope="col" class="px-3 py-2 font-medium">Auth</th>
				<th scope="col" class="px-3 py-2 font-medium">Priority</th>
				<th scope="col" class="px-3 py-2 font-medium">Status</th>
				<th scope="col" class="px-3 py-2 font-medium">Keys</th>
				<th scope="col" class="px-3 py-2 font-medium">Last test</th>
				<th scope="col" class="px-3 py-2 font-medium">Detail</th>
				{#if ondelete || selectable}
					<th scope="col" class="px-3 py-2 font-medium">
						<span class="sr-only">Actions</span>
					</th>
				{/if}
			</tr>
		</thead>
		<tbody>
			{#each endpoints as entry (entry.id)}
				<tr class="border-t border-[var(--color-border)]">
					<td data-label="Provider" class="px-3 py-2">{entry.provider_name ?? entry.provider_id}</td
					>
					<td data-label="Endpoint" class="px-3 py-2">
						<span>{entry.label}</span>
						{#if entry.tags?.length}
							<span class="ml-1 inline-flex flex-wrap gap-1 align-middle">
								{#each entry.tags as tag (tag)}
									<span
										class="rounded-[var(--radius-sm)] border border-[var(--color-border)] px-1 text-xs"
										>{tag}</span
									>
								{/each}
							</span>
						{/if}
					</td>
					<td data-label="Auth" class="px-3 py-2"
						>{AUTH_TYPE_LABELS[entry.auth_type] ?? entry.auth_type}</td
					>
					<td data-label="Priority" class="px-3 py-2 tabular-nums">{entry.priority}</td>
					<td data-label="Status" class="px-3 py-2">
						{statusLabel(entry.status)}
						{#if entry.status !== ENDPOINT_STATUS_ACTIVE && entry.disabled_until}
							<!-- The word, not only the colour: a parked account has to read as parked
								 without depending on hue. -->
							<span class="ml-1 text-xs text-[var(--color-text-muted)]"
								>until {new Date(entry.disabled_until).toLocaleString()}</span
							>
						{:else if entry.status !== ENDPOINT_STATUS_ACTIVE}
							<span class="ml-1 text-xs text-[var(--color-text-muted)]">switched off</span>
						{/if}
					</td>
					<td data-label="Keys" class="px-3 py-2 tabular-nums">
						{entry.active_key_count} of {entry.key_count} healthy
					</td>
					<td data-label="Last test" class="px-3 py-2">
						{#if entry.test_status}
							{entry.test_status.state} {entry.test_status.latency_ms}ms
						{:else}
							Not tested
						{/if}
					</td>
					<td data-label="Detail" class="px-3 py-2">
						<Button variant="ghost" class="underline" onclick={() => onopen(entry)}>Open</Button>
					</td>
					{#if ondelete || selectable}
						<!-- Selection sits with the action controls rather than in a column of its own: under
							 768px every cell becomes a labelled line, and a checkbox stranded on its own line
							 reads as a property of the row instead of one of the things you can do to it.
							 One flex line, one target size: two inline controls each riding its own baseline
							 is how the checkbox landed a few pixels off the delete glyph. -->
						<td data-label="Actions" class="px-3 py-2">
							<div class="flex flex-wrap items-center justify-end gap-1">
								{#if selectable}
									<label class="flex min-h-11 min-w-11 items-center justify-center">
										<input
											type="checkbox"
											class="size-4"
											aria-label={`Select ${entry.label} for bulk actions`}
											checked={isChecked(entry.id)}
											onchange={(event) => onselect?.(entry.id, event.currentTarget.checked)}
										/>
									</label>
								{/if}
								{#if ondelete}
									<Button
										variant="destructive-ghost"
										size="icon"
										aria-label="Delete"
										title="Delete"
										onclick={() => ondelete(entry)}
									>
										<DeleteIcon class="size-4" aria-hidden="true" />
									</Button>
								{/if}
							</div>
						</td>
					{/if}
				</tr>
			{/each}
		</tbody>
	</table>
</div>
