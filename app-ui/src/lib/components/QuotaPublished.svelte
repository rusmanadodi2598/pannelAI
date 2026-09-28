<script lang="ts">
	// One endpoint's published quota, asked for on demand (docs/SPEC-UI/001-SPEC-UI.md §6.6,
	// docs/SPEC-API/001-SPEC-API.md §7.12's published-read block, docs/DRAFT/036 §7).
	//
	// This is the third number the quota screen holds. The counted windows say what this gateway sent and
	// the cap says what the operator forbade; this says what the provider reports it has left, read live at
	// the moment the operator asked. A card that showed it beside the other two without saying so would
	// read as a correction of the gateway's own count, so the note above the rows is not decoration.
	//
	// It is asked for, never fetched on render: the read is one call per endpoint, the screen lists a
	// provider's endpoints inside one card, and the owner's stated scale is hundreds to thousands of keys
	// per provider. The cap editor refuses the same arithmetic for the same reason (SPEC-API §7.12).
	//
	// Amounts print exactly as the provider spelled them. They arrive as decimal strings because a credit
	// balance has no integer form, and re-rendering one through a number would either drop a fraction or
	// invent places the provider never claimed. Only the bar and the percentage parse them.
	import { getPublishedQuota } from '$lib/api/usage';
	import {
		PUBLISHED_QUOTA_NOTE,
		publishedQuotaNotice,
		publishedWindowView,
		type PublishedQuotaUsage
	} from '$lib/schemas/quota-published';
	import { quotaBar } from '$lib/schemas/quota';
	import { countdownText, formatTimestamp } from '$lib/utils/time';

	type Props = {
		/** The connection to ask about, which is also the key this instance's answer belongs to. */
		endpointId: string;
		/** The provider the endpoint's row names, which is who is being asked. */
		provider: string;
		/** The label the card shows for this endpoint, so the control says what it acts on (R-28). */
		label: string;
		/** The card's shared clock, which is what the countdown is measured against. */
		now: number;
	};

	let { endpointId, provider, label, now }: Props = $props();

	let usage = $state<PublishedQuotaUsage | null>(null);
	let loading = $state(false);
	let error = $state<string | null>(null);

	const notice = $derived(usage === null ? null : publishedQuotaNotice(usage));

	// The guard is the button's `disabled` in code as well as in the DOM: one press asks one read, and a
	// second that arrived while the first was in flight would leave two answers racing for one row.
	async function ask(): Promise<void> {
		if (loading) return;

		loading = true;
		error = null;
		const result = await getPublishedQuota(endpointId);
		loading = false;

		if (!result.ok) {
			// The refusal is the gateway's own sentence (no such endpoint, this family publishes
			// nothing, no credential to ask with), so it is shown rather than translated. The previous
			// answer is dropped with it: a stale balance under a fresh failure reads as still current.
			error = result.error.message;
			usage = null;
			return;
		}

		usage = result.data;
	}
</script>

<div class="flex flex-col gap-2 border-t border-[var(--color-border)] pt-2">
	<div class="flex flex-wrap items-center gap-2">
		<button
			type="button"
			class="inline-flex min-h-11 items-center rounded-[var(--radius-md)] border border-[var(--color-border)] px-3 text-sm disabled:opacity-50"
			aria-label={`Ask the provider about ${label}`}
			disabled={loading}
			onclick={() => void ask()}
		>
			Ask {provider}
		</button>
		{#if loading}
			<span role="status" class="text-xs text-[var(--color-text-muted)]">Asking the provider…</span>
		{/if}
	</div>

	{#if error !== null}
		<p role="alert" class="text-sm text-[var(--color-danger)]">{error}</p>
	{/if}

	{#if usage !== null}
		<section role="group" aria-label={`Published quota for ${label}`} class="flex flex-col gap-2">
			<p class="text-xs text-[var(--color-text-muted)]">{PUBLISHED_QUOTA_NOTE}</p>
			{#if usage.plan}
				<p class="text-sm">Plan: {usage.plan}</p>
			{/if}

			{#each usage.data as window (window.label)}
				{@const view = publishedWindowView(window)}
				{@const bar = quotaBar(view.used, view.limit)}
				<div class="flex flex-col gap-1">
					<div class="flex items-center justify-between gap-2 text-sm">
						<span class="truncate font-medium">{view.label}</span>
						<span class="shrink-0 tabular-nums">{view.percent}</span>
					</div>

					{#if bar !== null}
						<div class="h-2 overflow-hidden rounded-full bg-[var(--color-surface-3)]">
							<div
								class="h-full rounded-full"
								style={`width: ${bar.width}%; background: ${bar.color};`}
							></div>
						</div>
					{/if}

					<div
						class="flex flex-wrap items-center justify-between gap-x-3 gap-y-1 text-xs text-[var(--color-text-muted)]"
					>
						<span class="tabular-nums">
							{view.limitText === null ? view.usedText : `${view.usedText} / ${view.limitText}`}
						</span>
						{#if view.resetsAt}
							<span
								>Resets {countdownText(view.resetsAt, now)} (at
								{formatTimestamp(view.resetsAt)})</span
							>
						{/if}
					</div>
				</div>
			{/each}

			{#if notice !== null}
				<p class="text-sm">{notice}</p>
			{/if}

			<!-- A live read ages, so its instant travels with the numbers: "3000 left" is only true as of
			     the stamp beside it. -->
			<p class="text-xs tabular-nums text-[var(--color-text-muted)]">
				Asked {formatTimestamp(usage.fetched_at)}
			</p>
		</section>
	{/if}
</div>
