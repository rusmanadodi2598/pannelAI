<script lang="ts">
	// The quota cards (docs/SPEC-UI/001-SPEC-UI.md §6.6; reshaped 2026-09-25 to follow the reference's
	// ProviderLimits, owner ralat 2026-09-25): one card per endpoint, endpoints grouped by provider in
	// first-seen order, one progress row per window kind.
	//
	// Two conventions are kept from the screen's own rules rather than copied from the reference:
	// the printed percentage is percent USED (§6.6's "percent used", quotaPercentLabel), and the
	// counters print without a unit because the wire carries none. The bar's fill is the used share;
	// its colour is driven by the REMAINING share the reference colours on (above 70% remaining ok,
	// 30-70 warn, below danger) in the panel's own tokens.
	//
	// A window the gateway recorded without a provider (the credential-free lane's virtual endpoint)
	// cannot sit under a provider heading. It renders in its own group with a sentence saying the
	// counts are local, which is the reference's answer for a provider whose quota it cannot fetch
	// (the card `message` path) without hiding data the gateway did send.
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

	type Group = { provider: string; endpoints: { id: string; windows: QuotaWindow[] }[] };

	const groups = $derived.by<Group[]>(() => {
		const out: Group[] = [];
		for (const window of windows) {
			let group = out.find((candidate) => candidate.provider === window.provider_id);
			if (!group) {
				group = { provider: window.provider_id, endpoints: [] };
				out.push(group);
			}
			const card = group.endpoints.find((endpoint) => endpoint.id === window.endpoint_id);
			if (card) card.windows.push(window);
			else group.endpoints.push({ id: window.endpoint_id, windows: [window] });
		}
		return out;
	});

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

<div class="flex flex-col gap-4">
	{#each groups as group (group.provider)}
		<section class="flex flex-col gap-2" aria-label={group.provider || 'No provider'}>
			{#if group.provider}
				<h3 class="truncate text-sm font-medium">{group.provider}</h3>
			{:else}
				<h3 class="text-sm font-medium text-[var(--color-text-muted)]">No provider</h3>
				<p class="text-sm text-[var(--color-text-muted)]">
					Counted locally by this gateway; the provider behind this lane publishes no quota.
				</p>
			{/if}

			<div class="grid gap-3 md:grid-cols-2">
				{#each group.endpoints as endpoint (endpoint.id)}
					<div
						class="flex flex-col gap-3 rounded-[var(--radius-md)] border border-[var(--color-border)] p-4"
					>
						<h4 class="truncate text-sm font-semibold">{endpointLabel(endpoint.id)}</h4>
						{#each endpoint.windows as window (window.window)}
							{@const share = remainingShare(window)}
							<div class="flex flex-col gap-1">
								<div class="flex items-center justify-between gap-2 text-sm">
									<span class="flex items-center gap-2">
										<span class="font-medium">{window.window}</span>
										<span
											class="rounded-[var(--radius-sm)] bg-[var(--color-surface-2)] px-2 py-0.5 text-xs"
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
			</div>
		</section>
	{/each}

	<p class="text-sm text-[var(--color-text-muted)]">
		Source: computed means {QUOTA_SOURCE_EXPLANATIONS.computed} reported means
		{QUOTA_SOURCE_EXPLANATIONS.reported}
	</p>
</div>
