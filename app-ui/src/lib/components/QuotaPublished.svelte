<script lang="ts">
	// One connection's provider quota (docs/SPEC-UI/001-SPEC-UI.md §6.6, docs/SPEC-API/001-SPEC-API.md
	// §7.12's published-read block).
	//
	// This is the number the operator came for, so it is the first thing on the connection and it arrives
	// with the page: the collection read carries the provider's answer for every endpoint the page names,
	// and the card renders it without asking. What remains per-card is the operator's own press: `force`,
	// which asks that one provider now instead of rereading the worker's cache.
	//
	// Three things keep this number from being mistaken for the gateway's count: the note above the rows
	// saying which ledger it came from, the instant the PROVIDER said it, and a mark when the figure is the
	// poll worker's stored answer rather than a fresh one. A live number whose age is unstated cannot be
	// acted on, and a cached one whose staleness is hidden is worse. The instant dates the figures, so an
	// answer that carries only a sentence prints no instant rather than the placeholder its row holds.
	//
	// Amounts print exactly as the provider spelled them. They arrive as decimal strings because a credit
	// balance has no integer form, and re-rendering one through a number would either drop a fraction or
	// invent places the provider never claimed. Only the bar and the percentage parse them, and both reuse
	// the counted rows' geometry so one screen cannot hold two answers to "percent used".
	import { getPublishedQuota } from '$lib/api/usage';
	import {
		PUBLISHED_QUOTA_NOTE,
		publishedAmountText,
		publishedAskedStamp,
		publishedBar,
		publishedAttemptFailure,
		publishedQuotaNotice,
		publishedRefillVerb,
		publishedWasNeverPolled,
		publishedWindowView,
		type PublishedQuotaUsage
	} from '$lib/schemas/quota-published';
	import { countdownText, formatTimestamp } from '$lib/utils/time';

	type Props = {
		/** The connection this block speaks for, which is also the key the page's answer arrived under. */
		endpointId: string;
		/** The provider the endpoint's row names, which is who the control asks. */
		provider: string;
		/** The label the card shows for this endpoint, so the control says what it acts on (R-28). */
		label: string;
		/** The provider's answer the page read for this connection. A connection whose provider publishes
		 *  no quota has no answer and no block: the card does not render this component for it. */
		usage: PublishedQuotaUsage;
		/** The card's shared clock, which is what the countdown is measured against. */
		now: number;
	};

	let { endpointId, provider, label, usage, now }: Props = $props();

	// What this connection shows, until the page reads again. The card's own press owns an `override`, and
	// the override is keyed on the page entry it replaced: while that entry is still the one the page
	// holds, the press wins, and the next page read (a different object) replaces it automatically. That
	// is what keeps one connection's refresh from moving another's numbers, and keeps a forced answer from
	// outliving the read beside it.
	let override = $state<{
		of: PublishedQuotaUsage;
		usage: PublishedQuotaUsage | null;
		error: string | null;
	} | null>(null);
	let loading = $state(false);

	const shown = $derived(override !== null && override.of === usage ? override.usage : usage);
	const error = $derived(override !== null && override.of === usage ? override.error : null);
	const notice = $derived(shown === null ? null : publishedQuotaNotice(shown));
	const asked = $derived(shown === null ? null : publishedAskedStamp(shown));
	const failed = $derived(shown === null ? null : publishedAttemptFailure(shown));

	// Whether there is a provider figure to attribute. A capable account the worker has not reached arrives
	// marked `never_polled`, and this block's own press holds no figure while it is in flight. Both have
	// nothing to date, so the block prints "Not polled yet." and leaves off the ledger note and the "Asked"
	// stamp. An account behind a provider that publishes no quota never reaches this component at all.
	const neverPolled = $derived(publishedWasNeverPolled(shown));

	// The guard is the button's `disabled` in code as well as in the DOM: one press asks one read, and a
	// second that arrived while the first was in flight would leave two answers racing for one connection.
	async function askNow(): Promise<void> {
		if (loading) return;

		const askedAgainst = usage;
		loading = true;
		override = { of: askedAgainst, usage: null, error: null };
		const result = await getPublishedQuota(endpointId, { force: true });
		loading = false;

		if (!result.ok) {
			// The refusal is the gateway's own sentence (no such endpoint, this family publishes nothing,
			// no credential to ask with), so it is shown rather than translated. The previous number goes
			// with it: a stale balance under a fresh failure reads as still current.
			override = { of: askedAgainst, usage: null, error: result.error.message };
			return;
		}

		override = { of: askedAgainst, usage: result.data, error: null };
	}
</script>

<section role="group" aria-label={`Published quota for ${label}`} class="flex flex-col gap-2">
	{#if !neverPolled}
		<p class="text-xs text-[var(--color-text-muted)]">{PUBLISHED_QUOTA_NOTE}</p>
	{/if}

	{#if !neverPolled && shown?.plan}
		<p class="text-sm">Plan: {shown.plan}</p>
	{/if}

	<div class="flex flex-wrap items-center gap-2">
		<button
			type="button"
			class="inline-flex min-h-11 items-center rounded-[var(--radius-md)] border border-[var(--color-border)] px-3 text-sm disabled:opacity-50"
			aria-label={`Ask the provider about ${label}`}
			disabled={loading}
			onclick={() => void askNow()}
		>
			Ask {provider} now
		</button>
		{#if loading}
			<span role="status" class="text-xs text-[var(--color-text-muted)]">Asking the provider…</span>
		{/if}
	</div>

	{#if error !== null}
		<p role="alert" class="text-sm text-[var(--color-danger)]">{error}</p>
	{:else if neverPolled && !loading}
		<!-- A connection the worker has never answered shows the gap rather than an empty space, so a
		     missing number reads as never asked instead of as asked-and-answered-with-nothing. Muted, never an
		     error, and it does not claim the provider publishes nothing: those are different facts. A press
		     in flight is not that state either: the status line above already says the provider is being
		     asked, and this would describe the moment the card is leaving rather than the one it is in. -->
		<p class="text-sm text-[var(--color-text-muted)]">
			Not polled yet. {provider} has not been asked for this connection.
		</p>
	{/if}

	{#if !neverPolled && shown !== null}
		{#each shown.data as window (window.label)}
			{@const row = publishedWindowView(window)}
			{@const bar = publishedBar(row)}
			<div class="flex flex-col gap-1">
				<div class="flex items-center justify-between gap-2 text-sm">
					<span class="truncate font-medium">{row.label}</span>
					{#if row.creditBalance}
						<!-- A balance is a sum of money, so it does not wear the shape of a percentage. -->
						<span
							class="shrink-0 rounded-[var(--radius-sm)] border border-[var(--color-border)] px-2 py-0.5 text-xs"
							>{row.percent}</span
						>
					{:else}
						<span class="shrink-0 tabular-nums">{row.percent}</span>
					{/if}
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
					<span class="tabular-nums">{publishedAmountText(row)}</span>
					{#if row.resetsAt}
						<span
							>{publishedRefillVerb(row)}
							{countdownText(row.resetsAt, now)} (at
							{formatTimestamp(row.resetsAt)})</span
						>
					{/if}
				</div>
			</div>
		{/each}

		{#if failed !== null && shown.data.length > 0}
			<!-- The worker keeps the last good figures across a failing poll, which is the right call and
			     the wrong silence: these numbers are true as of their stamp and nobody has replaced them
			     since. Warn, not danger, because the read that produced them completed. -->
			<p class="text-xs text-[var(--color-warn)]">
				Last poll failed ({failed.count} in a row){#if failed.at !== null}, asked
					{formatTimestamp(failed.at)}{/if}. The figures above are the last the provider gave.
			</p>
		{/if}

		{#if notice !== null}
			<!-- A soft outcome is the provider's sentence, not a failure: the family publishes nothing, or
			     the credential answered with nothing to report. Danger styling would tell the operator the
			     read broke when it completed. -->
			<p class="text-sm text-[var(--color-text-muted)]">{notice}</p>
		{/if}

		<!-- The instant travels with the numbers: "3000 left" is only true as of the stamp beside it, and
		     the mark says whether that stamp is this press or the poll worker's last one. A soft answer has
		     no numbers and no real instant, so it prints its sentence alone. -->
		{#if asked !== null}
			<p class="text-xs tabular-nums text-[var(--color-text-muted)]">
				Asked {formatTimestamp(asked)}{shown.cached === true ? ' · cached' : ''}
			</p>
		{/if}
	{/if}
</section>
