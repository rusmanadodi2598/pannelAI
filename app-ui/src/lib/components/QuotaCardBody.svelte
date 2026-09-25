<script lang="ts">
	// One quota card's body (docs/PORT/005-PORT-QUOTA-CARDS.md D1/D6): the sentence the provider-less
	// lane opens with, then one row group per endpoint with its windows. Split out of QuotaCards so
	// the card (fold, checkbox, pagination) and the rows stay under the file-size cap on their own;
	// the scrolling wrapper stays in QuotaCards because `aria-controls` points at it.
	import { quotaPercentLabel, type QuotaWindow } from '$lib/schemas/quota';
	import { formatCount } from '$lib/schemas/usage-view';
	import { countdownText, formatTimestamp } from '$lib/utils/time';

	let {
		group,
		labels,
		now
	}: {
		group: { provider: string; endpoints: { id: string; windows: QuotaWindow[] }[] };
		labels: Map<string, string>;
		now: number;
	} = $props();

	function endpointLabel(id: string): string {
		return labels.get(id) ?? id;
	}

	// The remaining share the reference colours on. Null when no ceiling was published, so the row
	// prints the counter without a bar rather than inventing a full-width one.
	function remainingShare(window: QuotaWindow): number | null {
		if (window.limit === null || window.limit === undefined || window.limit <= 0) return null;
		return Math.max(0, 100 - Math.round((window.used / window.limit) * 100));
	}

	function barColor(share: number | null): string {
		if (share === null) return '';
		if (share > 70) return 'var(--color-ok)';
		if (share >= 30) return 'var(--color-warn)';
		return 'var(--color-danger)';
	}

	function counterText(window: QuotaWindow): string {
		if (window.limit === null || window.limit === undefined || window.limit <= 0) {
			return formatCount(window.used);
		}
		return `${formatCount(window.used)} / ${formatCount(window.limit)}`;
	}

	function usedShare(window: QuotaWindow): number {
		if (window.limit === null || window.limit === undefined || window.limit <= 0) return 0;
		return Math.min(100, Math.round((window.used / window.limit) * 100));
	}
</script>

{#if !group.provider}
	<p class="text-sm text-[var(--color-text-muted)]">
		Counted locally by this gateway; the provider behind this lane publishes no quota.
	</p>
{/if}
{#each group.endpoints as endpoint (endpoint.id)}
	<div class="flex flex-col gap-2">
		<h4 class="truncate text-sm font-semibold">{endpointLabel(endpoint.id)}</h4>
		{#each endpoint.windows as window (window.window)}
			{@const share = remainingShare(window)}
			<div class="flex flex-col gap-1">
				<div class="flex items-center justify-between gap-2 text-sm">
					<span class="flex items-center gap-2">
						<span class="font-medium">{window.window}</span>
						<span class="rounded-[var(--radius-sm)] bg-[var(--color-surface-2)] px-2 py-0.5 text-xs"
							>{window.source}</span
						>
					</span>
					<span class="tabular-nums">{quotaPercentLabel(window.used, window.limit)}</span>
				</div>

				{#if share !== null}
					<div class="h-2 overflow-hidden rounded-full bg-[var(--color-surface-3)]">
						<div
							class="h-full rounded-full"
							style={`width: ${usedShare(window)}%; background: ${barColor(share)};`}
						></div>
					</div>
				{/if}

				<div
					class="flex flex-wrap items-center justify-between gap-x-3 gap-y-1 text-xs text-[var(--color-text-muted)]"
				>
					<span class="tabular-nums">{counterText(window)}</span>
					{#if window.resets_at}
						<span
							>Resets {countdownText(window.resets_at, now)} (at
							{formatTimestamp(window.resets_at)})</span
						>
					{/if}
				</div>
			</div>
		{/each}
	</div>
{/each}
