<script lang="ts">
	// The upstream endpoint table (docs/SPEC-UI/001-SPEC-UI.md §6.2, tab 2).
	//
	// Columns follow the decision the operator makes here: which connection is failing and where it sits in
	// the routing order. Provider and endpoint come first to identify the row, priority and status decide
	// whether it needs attention, and the key and test columns say what the router last saw. Detail is a
	// button rather than a clickable row, so the action is reachable by keyboard and announced as a control.
	import { AUTH_TYPE_LABELS, ENDPOINT_STATUS_ACTIVE, type Endpoint } from '$lib/schemas/endpoint';

	let { endpoints, onopen }: { endpoints: Endpoint[]; onopen: (entry: Endpoint) => void } =
		$props();

	function statusLabel(status: string): string {
		return status === ENDPOINT_STATUS_ACTIVE ? 'Active' : status;
	}
</script>

<div class="overflow-x-auto rounded-[var(--radius-md)] border border-[var(--color-border)]">
	<table class="w-full min-w-[52rem] border-collapse text-sm">
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
			</tr>
		</thead>
		<tbody>
			{#each endpoints as entry (entry.id)}
				<tr class="border-t border-[var(--color-border)]">
					<td class="px-3 py-2">{entry.provider_name ?? entry.provider_id}</td>
					<td class="px-3 py-2">{entry.label}</td>
					<td class="px-3 py-2">{AUTH_TYPE_LABELS[entry.auth_type] ?? entry.auth_type}</td>
					<td class="px-3 py-2 tabular-nums">{entry.priority}</td>
					<td class="px-3 py-2">{statusLabel(entry.status)}</td>
					<td class="px-3 py-2 tabular-nums">
						{entry.active_key_count} of {entry.key_count} healthy
					</td>
					<td class="px-3 py-2">
						{#if entry.test_status}
							{entry.test_status.state} {entry.test_status.latency_ms}ms
						{:else}
							Not tested
						{/if}
					</td>
					<td class="px-3 py-2">
						<button type="button" class="min-h-11 underline" onclick={() => onopen(entry)}
							>Open</button
						>
					</td>
				</tr>
			{/each}
		</tbody>
	</table>
</div>
