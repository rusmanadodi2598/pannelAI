<script lang="ts">
	// The quota window table (docs/SPEC-UI/001-SPEC-UI.md §6.6).
	//
	// The source badge is functional rather than decorative (§6.6), so what the two values mean is stated
	// under the table instead of in a tooltip: a hover-only explanation is unavailable to a keyboard user and
	// to anyone reading on a tablet (§8.7.5).
	//
	// The endpoint column resolves a label from the endpoint list the screen already read, and falls back to
	// the identifier. That fallback is not a failure path to hide: the label map is bounded by the list's
	// first page, so an endpoint beyond it is named by its id, which is what the quota window itself carries.
	//
	// The counter has no unit on the wire, so the panel prints the number without one rather than choosing
	// between tokens and requests on the API's behalf.
	import {
		QUOTA_SOURCE_EXPLANATIONS,
		quotaPercentLabel,
		type QuotaWindow
	} from '$lib/schemas/quota';
	import { formatCount } from '$lib/schemas/usage-view';
	import { countdownText, formatTimestamp } from '$lib/utils/time';

	let {
		windows,
		labels,
		now
	}: { windows: QuotaWindow[]; labels: Map<string, string>; now: number } = $props();

	function endpointLabel(id: string): string {
		return labels.get(id) ?? id;
	}
</script>

<div class="flex flex-col gap-3">
	<div class="overflow-x-auto rounded-[var(--radius-md)] border border-[var(--color-border)]">
		<table class="w-full min-w-[56rem] border-collapse text-sm">
			<caption class="sr-only">Quota windows per endpoint, with their reset countdowns</caption>
			<thead class="bg-[var(--color-surface-2)] text-left">
				<tr>
					<th scope="col" class="px-3 py-2 font-medium">Provider</th>
					<th scope="col" class="px-3 py-2 font-medium">Endpoint</th>
					<th scope="col" class="px-3 py-2 font-medium">Window</th>
					<th scope="col" class="px-3 py-2 font-medium">Used</th>
					<th scope="col" class="px-3 py-2 font-medium">Limit</th>
					<th scope="col" class="px-3 py-2 font-medium">Percent used</th>
					<th scope="col" class="px-3 py-2 font-medium">Resets at</th>
					<th scope="col" class="px-3 py-2 font-medium">Source</th>
				</tr>
			</thead>
			<tbody>
				{#each windows as window (window.endpoint_id + window.window)}
					<tr class="border-t border-[var(--color-border)]">
						<td class="px-3 py-2">{window.provider_id}</td>
						<td class="px-3 py-2">{endpointLabel(window.endpoint_id)}</td>
						<td class="px-3 py-2">{window.window}</td>
						<td class="px-3 py-2 tabular-nums">{formatCount(window.used)}</td>
						<td class="px-3 py-2 tabular-nums">
							{#if window.limit === null || window.limit === undefined}
								<span class="text-[var(--color-text-muted)]">Not reported</span>
							{:else}
								{formatCount(window.limit)}
							{/if}
						</td>
						<td class="px-3 py-2 tabular-nums">{quotaPercentLabel(window.used, window.limit)}</td>
						<td class="px-3 py-2">
							{#if window.resets_at}
								{formatTimestamp(window.resets_at)}
								<span class="text-[var(--color-text-muted)]"
									>({countdownText(window.resets_at, now)})</span
								>
							{:else}
								<span class="text-[var(--color-text-muted)]">Not reported</span>
							{/if}
						</td>
						<td class="px-3 py-2">
							<span class="rounded-[var(--radius-sm)] bg-[var(--color-surface-2)] px-2 py-0.5"
								>{window.source}</span
							>
						</td>
					</tr>
				{/each}
			</tbody>
		</table>
	</div>

	<p class="text-sm text-[var(--color-text-muted)]">
		Source: computed means {QUOTA_SOURCE_EXPLANATIONS.computed} reported means
		{QUOTA_SOURCE_EXPLANATIONS.reported}
	</p>
</div>
