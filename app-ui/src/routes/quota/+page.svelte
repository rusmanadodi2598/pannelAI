<script lang="ts">
	// Quota Tracker (docs/SPEC-UI/001-SPEC-UI.md §6.6).
	//
	// The countdown ticks locally and the data refetches on an interval (§8.6.1). One timer drives both:
	// every second it moves `now`, which redraws the countdowns, and it reads again when the interval has
	// elapsed. `pollDue` holds that decision, so "paused" and "the tab is hidden" are rules with a test
	// rather than conditions scattered through a component.
	//
	// The read is paged over provider groups (docs/PORT/006-PORT-QUOTA-PAGING.md D1/D5): the screen asks
	// for its card page size, and the pager walks server pages. A read that discovers the data shrank
	// below the page being read parks on the last page there is (`clampPage`) and reads that page, so a
	// stranded page can never sit over an empty table.
	//
	// The endpoint labels come from the first page of the endpoint list, which is the only list route the API
	// offers. That read is best effort: if it fails the table still renders, naming endpoints by their ids,
	// and says why.
	//
	// The provider's own numbers arrive on the same read as the windows (`published`), so a card shows what
	// its connection has left without a request of its own and without the operator pressing anything. The
	// page keys them by endpoint and hands them down; a card that wants a fresher answer asks for it itself.
	import { resolve } from '$app/paths';
	import { onMount } from 'svelte';
	import QuotaCaps from '$lib/components/QuotaCaps.svelte';
	import QuotaCards from '$lib/components/QuotaCards.svelte';
	import QuotaToolbar from '$lib/components/QuotaToolbar.svelte';
	import StateMessage from '$lib/components/StateMessage.svelte';
	import { listEndpointLabels } from '$lib/api/endpoints';
	import { listQuotaWindows } from '$lib/api/usage';
	import {
		clampPage,
		pollDue,
		pollSecondsRemaining,
		QUOTA_PAGE_SIZE,
		QUOTA_POLL_MS
	} from '$lib/polling';
	import type { PublishedQuotaUsage } from '$lib/schemas/quota-published';
	import { quotaCardGroups, quotaCardProviders, type QuotaWindow } from '$lib/schemas/quota';
	import { formatTimestamp } from '$lib/utils/time';

	// The countdown's resolution. A second is the smallest unit the countdown prints, so a faster tick would
	// redraw without changing what anyone reads.
	const TICK_MS = 1000;

	let windows = $state<QuotaWindow[]>([]);
	let published = $state<PublishedQuotaUsage[]>([]);
	let publishedNote = $state<string | null>(null);
	let labels = $state(new Map<string, string>());
	let loading = $state(true);
	let busy = $state(false);
	// A press that landed while a read was in flight. Remembered rather than swallowed, so the refresh
	// control is honest about having been pressed even when the interval read is mid-flight.
	let rerunRequested = $state(false);
	let error = $state<string | null>(null);
	let labelNotice = $state<string | null>(null);
	let paused = $state(false);
	let now = $state(Date.now());
	let lastLoadedAt = $state(0);
	let readAt = $state('');
	let page = $state(1);
	let totalGroups = $state(0);
	let providerFilter = $state('');

	// Every endpoint the window table names, so the cap picker can offer one the label list's first page
	// does not cover. A cap belongs to an endpoint, and the window table is the other place the screen
	// learns one exists.
	const windowEndpointIds = $derived(windows.map((window) => window.endpoint_id));

	// The providers this page's cards name, in first-seen order, off the same union the cards group by: a
	// provider with an account but no counted window is still a choice the filter can make. The lane whose
	// windows carry no provider is not such a choice, and empty is the value "all providers" already holds.
	const providerIds = $derived(quotaCardProviders(quotaCardGroups(windows, published)));

	// The pager the cards render is the gateway's page count: meta.total counts provider groups, and
	// the screen's card page size is the read's per_page.
	const pageCount = $derived(Math.max(1, Math.ceil(totalGroups / QUOTA_PAGE_SIZE)));

	const secondsToNext = $derived(
		pollSecondsRemaining({ paused, lastLoadedAt }, now, QUOTA_POLL_MS)
	);

	onMount(() => {
		const timer = setInterval(tick, TICK_MS);
		document.addEventListener('visibilitychange', onVisibility);
		void load();

		return () => {
			clearInterval(timer);
			document.removeEventListener('visibilitychange', onVisibility);
		};
	});

	function tick(): void {
		now = Date.now();
		if (pollDue({ paused, lastLoadedAt }, now, QUOTA_POLL_MS, document.visibilityState))
			void load();
	}

	function onVisibility(): void {
		// Coming back to the tab reads once. The interval that elapsed while it was hidden was skipped rather
		// than queued, so this is what brings the table up to date again.
		if (document.visibilityState === 'visible') void load();
	}

	function goToPage(next: number): void {
		const bounded = Math.min(pageCount, Math.max(1, next));
		if (bounded === page) return;
		page = bounded;
		// The filter names providers on the page being left. Carrying it across a turn would show an empty
		// card list under a provider the new page may not have (the same reasoning as the bulk selection).
		providerFilter = '';
		void load();
	}

	async function load(): Promise<void> {
		// One read at a time. The interval and the manual control can otherwise overlap, and the slower
		// response would win the race for no reason. A press that lands mid-read asks for one more read
		// once this one lands.
		if (busy) {
			rerunRequested = true;
			return;
		}
		busy = true;
		try {
			const [quotaResult, endpointResult] = await Promise.all([
				listQuotaWindows({ page, per_page: QUOTA_PAGE_SIZE }),
				listEndpointLabels({ page: 1, per_page: 100 })
			]);

			lastLoadedAt = Date.now();
			readAt = new Date(lastLoadedAt).toISOString();

			if (endpointResult.ok) {
				labelNotice = null;
				labels = new Map(endpointResult.data.data.map((endpoint) => [endpoint.id, endpoint.label]));
			} else {
				labelNotice = `Endpoint labels could not be read, so ids show. ${endpointResult.error.message}`;
			}

			if (!quotaResult.ok) {
				error = quotaResult.error.message;
				return;
			}

			error = null;
			windows = quotaResult.data.data;
			published = quotaResult.data.published;
			publishedNote = quotaResult.data.published_note ?? null;
			totalGroups = quotaResult.data.meta.total;

			// The data can shrink between reads (a poll that answers fewer provider groups than the page
			// being read assumes). Park on the last page there is, then let the remembered rerun read that
			// page, so the cards always match the page indicator above them.
			const clamped = clampPage(page, totalGroups, QUOTA_PAGE_SIZE);
			if (clamped !== page) {
				page = clamped;
				rerunRequested = true;
				return;
			}
		} finally {
			busy = false;
			loading = false;
			if (rerunRequested) {
				rerunRequested = false;
				void load();
			}
		}
	}
</script>

<section class="flex flex-col gap-5">
	<div class="flex flex-col gap-1">
		<h1 class="text-lg font-semibold tracking-tight">Quota Tracker</h1>
		<p class="text-sm text-[var(--color-text-muted)]">
			Window spend per provider, and when it reopens.
		</p>
	</div>

	<QuotaToolbar
		{paused}
		{readAt}
		{secondsToNext}
		providers={providerIds}
		provider={providerFilter}
		onproviderchange={(next) => (providerFilter = next)}
		onrefresh={() => void load()}
		ontogglepause={() => (paused = !paused)}
	/>

	{#if labelNotice}
		<p role="status" class="text-sm text-[var(--color-text-muted)]">{labelNotice}</p>
	{/if}

	{#if loading}
		<StateMessage kind="loading" title="Loading quota windows" />
	{:else if error && windows.length === 0 && published.length === 0}
		<StateMessage kind="error" title="Quota windows could not be loaded" description={error}>
			{#snippet action()}
				<button type="button" class="underline" onclick={() => void load()}>Try again</button>
			{/snippet}
		</StateMessage>
	{:else if windows.length === 0 && published.length === 0}
		<StateMessage
			kind="empty"
			title="No quota windows yet"
			description="No traffic yet. Send one request, then rows appear here."
		>
			{#snippet action()}
				<a href={resolve('/endpoint-keys')} class="underline">Open Endpoint &amp; Key</a>
			{/snippet}
		</StateMessage>
	{:else}
		{#if error}
			<p role="alert" class="text-sm text-[var(--color-danger)]">
				{error} Showing the last read at {formatTimestamp(readAt)}.
			</p>
		{/if}

		<QuotaCards
			{windows}
			{labels}
			{published}
			{publishedNote}
			provider={providerFilter}
			{now}
			{page}
			{pageCount}
			onpagechange={goToPage}
		/>
	{/if}

	<!-- Outside the branch above: a cap is legal before the first routed request, so this section is the
	     one part of the screen that is useful on a gateway whose windows are all still empty. -->
	<div class="flex flex-col gap-3">
		<h2 class="text-base font-medium">Budget caps</h2>
		<p class="text-sm text-[var(--color-text-muted)]">
			Monthly ceiling per endpoint, in USD or tokens.
		</p>
		<QuotaCaps {labels} {windowEndpointIds} labelsUnread={labelNotice !== null} />
	</div>
</section>
