<script lang="ts">
	// The request log table (docs/SPEC-UI/001-SPEC-UI.md §6.11, tab 1).
	//
	// Columns follow §6.11's list. `has_bodies` is shown as a column rather than as a marker on a row,
	// because "capture was on and has a body" and "capture was on but nothing was stored" are different
	// facts and the operator should not have to open a row to learn which.
	//
	// Detail is a button rather than a clickable row, so the action is reachable by keyboard and announced
	// as a control.
	import { REQUEST_STATUS_LABELS, type RequestStatus } from '$lib/schemas/primitives';
	import type { LogRecord } from '$lib/schemas/log';
	import { formatCount } from '$lib/schemas/usage-view';
	import { formatTimestamp } from '$lib/utils/time';

	let { records, onopen }: { records: LogRecord[]; onopen: (record: LogRecord) => void } = $props();

	function statusLabel(status: RequestStatus): string {
		return REQUEST_STATUS_LABELS[status] ?? status;
	}
</script>

<div class="overflow-x-auto rounded-[var(--radius-md)] border border-[var(--color-border)]">
	<table class="w-full min-w-[64rem] border-collapse text-sm">
		<caption class="sr-only">Requests recorded in this window, newest first</caption>
		<thead class="bg-[var(--color-surface-2)] text-left">
			<tr>
				<th scope="col" class="px-3 py-2 font-medium">Timestamp</th>
				<th scope="col" class="px-3 py-2 font-medium">Request ID</th>
				<th scope="col" class="px-3 py-2 font-medium">Gateway key</th>
				<th scope="col" class="px-3 py-2 font-medium">Model</th>
				<th scope="col" class="px-3 py-2 font-medium">Endpoint</th>
				<th scope="col" class="px-3 py-2 font-medium">Latency</th>
				<th scope="col" class="px-3 py-2 font-medium">Status</th>
				<th scope="col" class="px-3 py-2 font-medium">Error</th>
				<th scope="col" class="px-3 py-2 font-medium">Bodies</th>
				<th scope="col" class="px-3 py-2 font-medium">Detail</th>
			</tr>
		</thead>
		<tbody>
			{#each records as record (record.request_id)}
				<tr class="border-t border-[var(--color-border)]">
					<td class="px-3 py-2">{formatTimestamp(record.ts)}</td>
					<td class="px-3 py-2">{record.request_id}</td>
					<td class="px-3 py-2">{record.gateway_key_id ?? 'Not recorded'}</td>
					<td class="px-3 py-2">{record.model ?? 'Not recorded'}</td>
					<td class="px-3 py-2">{record.endpoint_id ?? 'Not recorded'}</td>
					<td class="px-3 py-2 tabular-nums">{formatCount(record.latency_ms)} ms</td>
					<td class="px-3 py-2">{statusLabel(record.status)}</td>
					<td class="px-3 py-2">
						{#if record.error}
							{record.error}
						{:else}
							<span class="text-[var(--color-text-muted)]">None</span>
						{/if}
					</td>
					<td class="px-3 py-2">
						{#if record.has_bodies}
							Yes
						{:else}
							<span class="text-[var(--color-text-muted)]">No</span>
						{/if}
					</td>
					<td class="px-3 py-2">
						<button type="button" class="min-h-11 underline" onclick={() => onopen(record)}
							>Open</button
						>
					</td>
				</tr>
			{/each}
		</tbody>
	</table>
</div>
