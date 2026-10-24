<script lang="ts">
	// The raw usage record table (docs/SPEC-UI/001-SPEC-UI.md §6.5, tab 2).
	//
	// Columns follow §6.5's list. Tokens are shown as in and out in one column, because the two numbers are
	// read together and a third and fourth cache column would push the status off a laptop screen; the cache
	// counts are in the row's detail.
	//
	// Detail is a button rather than a clickable row, so the action is reachable by keyboard and announced
	// as a control.
	import { Button } from '$lib/primitives/button/index.js';
	import { REQUEST_STATUS_LABELS, type RequestStatus } from '$lib/schemas/primitives';
	import type { UsageRecord } from '$lib/schemas/usage';
	import { formatCount } from '$lib/schemas/usage-view';
	import { formatTimestamp } from '$lib/utils/time';

	let { records, onopen }: { records: UsageRecord[]; onopen: (record: UsageRecord) => void } =
		$props();

	function statusLabel(status: RequestStatus): string {
		return REQUEST_STATUS_LABELS[status] ?? status;
	}
</script>

<div
	class="mobile-key-value-frame overflow-x-auto rounded-[var(--radius-md)] border border-[var(--color-border)]"
>
	<table class="mobile-key-value-table w-full border-collapse text-sm">
		<caption class="sr-only">Requests recorded in this window, newest first</caption>
		<thead class="bg-[var(--color-surface-2)] text-left">
			<tr>
				<th scope="col" class="px-3 py-2 font-medium">Timestamp</th>
				<th scope="col" class="px-3 py-2 font-medium">Request ID</th>
				<th scope="col" class="px-3 py-2 font-medium">Model</th>
				<th scope="col" class="px-3 py-2 font-medium">Provider</th>
				<th scope="col" class="px-3 py-2 font-medium">Endpoint</th>
				<th scope="col" class="px-3 py-2 font-medium">Gateway key</th>
				<th scope="col" class="px-3 py-2 font-medium">Tokens in / out</th>
				<th scope="col" class="px-3 py-2 font-medium">Cost (USD)</th>
				<th scope="col" class="px-3 py-2 font-medium">Latency</th>
				<th scope="col" class="px-3 py-2 font-medium">Status</th>
				<th scope="col" class="px-3 py-2 font-medium">Error code</th>
				<th scope="col" class="px-3 py-2 font-medium">Detail</th>
			</tr>
		</thead>
		<tbody>
			{#each records as record (record.id)}
				<tr class="border-t border-[var(--color-border)]">
					<td data-label="Timestamp" class="whitespace-nowrap px-3 py-2"
						>{formatTimestamp(record.ts)}</td
					>
					<td data-label="Request ID" class="px-3 py-2">{record.request_id}</td>
					<td data-label="Model" class="px-3 py-2">{record.model}</td>
					<td data-label="Provider" class="px-3 py-2">{record.provider_id}</td>
					<td data-label="Endpoint" class="px-3 py-2">{record.endpoint_id ?? 'Not recorded'}</td>
					<td data-label="Gateway key" class="px-3 py-2"
						>{record.gateway_key_id ?? 'Not recorded'}</td
					>
					<td data-label="Tokens in / out" class="px-3 py-2 tabular-nums">
						{formatCount(record.tokens_in)} / {formatCount(record.tokens_out)}
					</td>
					<td data-label="Cost (USD)" class="px-3 py-2 tabular-nums">{record.cost_usd}</td>
					<td data-label="Latency" class="px-3 py-2 tabular-nums"
						>{formatCount(record.latency_ms)} ms</td
					>
					<td data-label="Status" class="px-3 py-2">{statusLabel(record.status)}</td>
					<td data-label="Error code" class="px-3 py-2">
						{#if record.error_code}
							{record.error_code}
						{:else}
							<span class="text-[var(--color-text-muted)]">None</span>
						{/if}
					</td>
					<td data-label="Detail" class="px-3 py-2">
						<Button
							variant="ghost"
							class="underline"
							aria-label={`Open request ${record.request_id}`}
							onclick={() => onopen(record)}>Open</Button
						>
					</td>
				</tr>
			{/each}
		</tbody>
	</table>
</div>
