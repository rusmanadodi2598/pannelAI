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
	import { listProviders } from '$lib/api/providers';
	import { freshActive, liveMerge, streamLabel, type LiveView } from '$lib/schemas/usage-live-view';
	import { configuredProviders } from '$lib/schemas/usage-topology-view';
	import { openUsageLive, type LiveReport, type UsageLiveController } from '$lib/usage-live';
	import { formatTimestamp } from '$lib/utils/time';

	// How often the staleness guard is re-evaluated while something is in flight. The gateway sends frames
	// when something changes, and a keepalive is not a frame, so a request that hangs would otherwise stay
	// on screen until the next frame arrived, which could be never. The timer exists only while a request
	// is in flight, so an idle screen runs no clock at all.
	const ACTIVE_TICK_MS = 1_000;

	let providers = $state<{ id: string; name: string }[]>([]);
	let providersNotice = $state<string | null>(null);
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
				return 'Live updates are paused. The totals below are from the last completed read.';
			case 'unavailable':
				return `${report.reason ?? 'The live stream is unavailable.'} ${
					report.retrying
						? 'Another attempt is scheduled.'
						: 'Retrying has stopped. Use Try again to reconnect.'
				}`;
			default:
				return 'Live updates are not running. They start when this tab is visible.';
		}
	});

	function providerName(id: string): string {
		const match = providers.find((provider) => provider.id.toLowerCase() === id.toLowerCase());
		return match?.name ?? id;
	}

	async function loadProviders(): Promise<void> {
		const result = await listProviders({ per_page: 100 });

		if (!result.ok) {
			providersNotice = `Providers could not be read (${result.error.message}), so the drawing has no nodes.`;
			return;
		}

		const { data, meta } = result.data;
		providersNotice =
			meta.total > data.length
				? `The registry carries ${meta.total} providers and this drawing reads the first ${data.length}.`
				: null;
		providers = configuredProviders(data);
	}

	onMount(() => {
		void loadProviders();

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
			What the gateway is routing right now, from its live stream. The totals and charts below come
			from their own reads and are not changed by anything here.
		</p>
	</div>

	<div class="flex flex-wrap items-center gap-3">
		<span
			class="rounded-[var(--radius-sm)] border border-[var(--color-border)] px-2 py-1 text-sm"
			role="status"
			aria-live="polite">{streamLabel(report.status)}</span
		>

		<button
			type="button"
			aria-pressed={paused}
			class="min-h-11 rounded-[var(--radius-sm)] border border-[var(--color-border)] px-3 text-sm aria-pressed:bg-[var(--color-surface-2)]"
			onclick={() => (paused ? controller?.resume() : controller?.pause())}
			>{paused ? 'Resume live updates' : 'Pause live updates'}</button
		>

		{#if report.status === 'unavailable'}
			<button
				type="button"
				class="min-h-11 rounded-[var(--radius-sm)] border border-[var(--color-border)] px-3 text-sm"
				onclick={() => controller?.retry()}>Try again</button
			>
		{/if}
	</div>

	<p class="text-sm text-[var(--color-text-muted)]">{statusLine}</p>

	{#if providersNotice}
		<p role="status" class="text-sm text-[var(--color-text-muted)]">{providersNotice}</p>
	{/if}

	<!-- `live` is what the drawing needs to move: motion says "happening now", so it is reserved for a
	     connection frames are arriving on. A paused or dropped stream leaves the last known state on
	     screen in colour and stops every moving part (draft 013 F2). -->
	<UsageTopology {providers} {active} {last} {error} live={report.status === 'live'} />

	<!-- Rendered only when the frame carries finished requests. The absence is not silent: the drawing's
	     summary states that nothing has finished since the screen opened. -->
	{#if recent.length > 0}
		<UsageRecentList {recent} {providerName} />
	{/if}
</section>
