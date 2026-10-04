<script lang="ts">
	// The live half of the Usage overview (docs/DRAFT/012-USAGE-LIVE-UI-READINESS.md F2 and F3).
	//
	// The division of labour is the point of this component. The totals and the charts on this tab come
	// from the REST reads and are never touched by anything here. What arrives on the stream is in flight
	// requests, the requests that just finished, and the provider the gateway last errored on, and those
	// live in a type (`LiveView`) that has no aggregate field to write into. The reference fork draws the
	// same line, and it is what keeps "the panel computes nothing it cannot cite" true of a live screen.
	//
	// The status line is where R-36 is answered: the stream is called live only while frames are arriving,
	// and every other state says what it is instead. The panel reads the stream by `fetch`, so it knows the
	// difference between a route that is not served and a connection that dropped, and it says which.
	import { onMount } from 'svelte';
	import UsageRecentList from '$lib/components/UsageRecentList.svelte';
	import UsageTopology from '$lib/components/UsageTopology.svelte';
	import { listCombos } from '$lib/api/combos';
	import { listProviders } from '$lib/api/providers';
	import {
		freshActive,
		liveFacts,
		liveMerge,
		providerDisplayName,
		streamLabel,
		type LiveView
	} from '$lib/schemas/usage-live-view';
	import { configuredCombos, configuredProviders } from '$lib/schemas/usage-topology-nodeset';
	import { openUsageLive, type LiveReport, type UsageLiveController } from '$lib/usage-live';
	import { formatTimestamp } from '$lib/utils/time';

	// How often the staleness guard is re-evaluated while something is in flight. The gateway sends frames
	// when something changes, and a keepalive is not a frame, so a request that hangs would otherwise stay
	// on screen until the next frame arrived, which could be never. The timer exists only while a request
	// is in flight, so an idle screen runs no clock at all.
	const ACTIVE_TICK_MS = 1_000;

	// The one box every tab on this row wears. The two controls already carried it; the owner's
	// correction of 2026-09-27 asked the chip and the new facts tab to join them rather than sit at
	// their own text size (draft 035 F1), and 44px is the panel's floor for a control's touch target
	// (SPEC-UI §8.6 rule 3).
	const TAB_BOX =
		'inline-flex min-h-11 items-center rounded-[var(--radius-sm)] border border-[var(--color-border)] px-3 text-sm';

	let providers = $state<{ id: string; name: string }[]>([]);
	let providersNotice = $state<string | null>(null);
	let combos = $state<{ id: string; name: string }[]>([]);
	let combosNotice = $state<string | null>(null);
	let live = $state<LiveView | null>(null);
	let report = $state<LiveReport>({ status: 'idle', reason: null, retrying: false });
	let now = $state(Date.now());

	let controller: UsageLiveController | null = null;

	const paused = $derived(report.status === 'paused');
	const active = $derived(freshActive(live?.active ?? [], now));
	const recent = $derived(live?.recent ?? []);
	const last = $derived(live?.recent[0]?.provider_id ?? '');
	const error = $derived(live?.errorProvider ?? '');

	const hasActive = $derived(active.length > 0);
	const facts = $derived(liveFacts(providers, active, last, error));

	$effect(() => {
		if (!hasActive) return;
		const timer = setInterval(() => {
			now = Date.now();
		}, ACTIVE_TICK_MS);
		return () => clearInterval(timer);
	});

	const statusLine = $derived.by(() => {
		switch (report.status) {
			case 'live':
				return live === null
					? 'Reading the gateway live stream.'
					: `Reading the gateway live stream. Last frame ${formatTimestamp(new Date(live.receivedAt).toISOString())}.`;
			case 'connecting':
				return 'Connecting to the gateway live stream.';
			case 'paused':
				return 'Live updates are paused. Totals are from the last read.';
			case 'unavailable':
				return report.reason ?? 'The live stream is unavailable.';
			default:
				return 'Live updates are not running. They start when this tab is visible.';
		}
	});

	function providerName(id: string): string {
		return providerDisplayName(providers, id);
	}

	async function loadProviders(): Promise<void> {
		const result = await listProviders({ per_page: 100 });

		if (!result.ok) {
			providersNotice = `Providers could not be read. The drawing has no nodes. ${result.error.message}`;
			return;
		}

		const { data, meta } = result.data;
		providersNotice =
			meta.total > data.length
				? `Shows the first ${data.length} of ${meta.total} providers.`
				: null;
		providers = configuredProviders(data);
	}

	// The combos are the drawing's other node set, and they are read rather than derived: the frame names
	// the combo a request is addressing, but only the combo list says which names exist at all, which is
	// what puts an idle combo on the band before anything routes through it.
	async function loadCombos(): Promise<void> {
		const result = await listCombos({ per_page: 100 });

		if (!result.ok) {
			combosNotice = `Combos could not be read. The drawing has no combo band. ${result.error.message}`;
			return;
		}

		const { data, meta } = result.data;
		combos = configuredCombos(data);
		combosNotice =
			meta.total > combos.length
				? `Shows the first ${combos.length} of ${meta.total} combos.`
				: null;
	}

	// One sentence about what the drawing left out, whichever half left it out: an operator counting nodes
	// on screen against a list they know should not have to work out which read they are looking at.
	const nodeNotice = $derived(
		[providersNotice, combosNotice].filter((part) => part !== null).join(' ') || null
	);

	onMount(() => {
		void loadProviders();
		void loadCombos();

		controller = openUsageLive({
			onFrame: (frame, receivedAt) => {
				live = liveMerge(live, frame, receivedAt);
			},
			onReport: (next) => {
				report = next;
			}
		});

		return () => {
			controller?.stop();
			controller = null;
		};
	});
</script>

<section class="flex flex-col gap-3">
	<div class="flex flex-col gap-1">
		<h2 class="text-base font-medium">Live routing</h2>
		<p class="text-sm text-[var(--color-text-muted)]">
			What the gateway routes right now, from its live stream.
		</p>
	</div>

	<div class="flex flex-wrap items-stretch gap-3">
		<span class={TAB_BOX} role="status" aria-live="polite">{streamLabel(report.status)}</span>

		<!-- eslint-disable svelte/no-useless-mustaches -- label and value sit in adjacent spans and
		     Svelte drops the whitespace between elements on separate lines, so each `{' '}` below is the
		     space the sentence needs when a screen reader reads its text content (draft 035, review
		     line 30). A flex container lays whitespace-only runs out as nothing, so the `gap-1` spacing
		     is unchanged. -->
		<!-- The live facts, stated in the row rather than in the drawing's frame (draft 035 F2): a
		     statement in the matched tab box, not a control. It wraps inside its own box and shrinks
		     below its content (`min-w-0`), so a long joined provider list breaks at the row's edge
		     instead of widening the page (review lines 1 and 31). No fact means no tab at all, which
		     keeps the owner's rule that a screen states what happened and never what did not
		     (2026-09-23). -->
		{#if facts.length > 0}
			<span class={`${TAB_BOX} min-w-0 flex-wrap gap-1`}>
				{#each facts as fact, index (fact.label)}
					{#if index > 0}
						{' '}<span aria-hidden="true">·</span>{' '}
					{/if}
					<span class="text-[var(--color-text-muted)]">{fact.label}:</span>
					{' '}<span class={`break-words ${fact.tone === 'status' ? 'text-[var(--color-ok)]' : ''}`}
						>{fact.value}</span
					>
				{/each}
			</span>
		{/if}

		<button
			type="button"
			aria-pressed={paused}
			class={`${TAB_BOX} hover:bg-[var(--color-surface-2)] aria-pressed:bg-[var(--color-surface-2)]`}
			onclick={() => (paused ? controller?.resume() : controller?.pause())}
			>{paused ? 'Resume live updates' : 'Pause live updates'}</button
		>

		{#if report.status === 'unavailable'}
			<button
				type="button"
				class={`${TAB_BOX} hover:bg-[var(--color-surface-2)]`}
				onclick={() => controller?.retry()}>Try again</button
			>
		{/if}
	</div>

	<p class="text-sm text-[var(--color-text-muted)]">{statusLine}</p>

	{#if nodeNotice}
		<p role="status" class="text-sm text-[var(--color-text-muted)]">{nodeNotice}</p>
	{/if}

	<!-- `live` is what the drawing needs to move: motion says "happening now", so it is reserved for a
	     connection frames are arriving on. A paused or dropped stream leaves the last known state on
	     screen in colour and stops every moving part (draft 013 F2). -->
	<UsageTopology {providers} {combos} {active} {last} {error} live={report.status === 'live'} />

	<!-- Rendered only when the frame carries finished requests. While none has, the screen's statement of
	     that is the drawing's colours and the status chip, not a sentence (owner's correction,
	     2026-09-23). -->
	{#if recent.length > 0}
		<UsageRecentList {recent} {providerName} />
	{/if}
</section>
