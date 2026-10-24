<script lang="ts">
	// The live half of the Usage overview.
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
	//
	// The provider registry is read once by the Overview and passed down. The drawing's providers and the
	// breakdown's names come from that one read, so a failed read produces one notice rather than two.
	import { onMount } from 'svelte';
	import ApiErrorText from '$lib/components/ApiErrorText.svelte';
	import type { ApiError } from '$lib/api/errors';
	import UsageLiveTabs from '$lib/components/UsageLiveTabs.svelte';
	import UsageTopology from '$lib/components/UsageTopology.svelte';
	import { listCombos } from '$lib/api/combos';
	import type { Provider } from '$lib/schemas/provider';
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
	import { Button } from '$lib/primitives/button/index.js';
	import { formatTimestampWithTime } from '$lib/utils/time';

	// How often the staleness guard is re-evaluated while something is in flight. The gateway sends frames
	// when something changes, and a keepalive is not a frame, so a request that hangs would otherwise stay
	// on screen until the next frame arrived, which could be never. The timer exists only while a request
	// is in flight, so an idle screen runs no clock at all.
	const ACTIVE_TICK_MS = 1_000;

	/** A read problem the panel states under its header, with the API error when there is one. */
	type Notice = { text: string; apiError?: ApiError };

	type Props = {
		/** The full registry, read once by the Overview and shared with its charts and breakdown. */
		providers: Provider[];
		/** The registry read's notice, including a paginated read the drawing cannot show in full. */
		providersNotice: Notice | null;
	};

	let { providers, providersNotice }: Props = $props();

	let combos = $state<{ id: string; name: string }[]>([]);
	let combosNotice = $state<Notice | null>(null);
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
	const drawingProviders = $derived(configuredProviders(providers));

	// The marker reports the connection's real state. It is paired with the state label, so the colour is
	// never the only statement.
	const STATUS_MARKER: Record<LiveReport['status'], string> = {
		idle: 'bg-[var(--color-border)]',
		connecting: 'bg-[var(--color-warn)]',
		live: 'bg-[var(--color-ok)]',
		paused: 'bg-[var(--color-warn)]',
		unavailable: 'bg-[var(--color-danger)]'
	};

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
					: `Reading the gateway live stream. Last frame ${formatTimestampWithTime(new Date(live.receivedAt).toISOString())}.`;
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

	// The combos are the drawing's other node set, and they are read rather than derived: the frame names
	// the combo a request is addressing, but only the combo list says which names exist at all, which is
	// what puts an idle combo on the band before anything routes through it.
	async function loadCombos(): Promise<void> {
		const result = await listCombos({ per_page: 100 });

		if (!result.ok) {
			combosNotice = {
				text: 'Combos could not be read. The drawing has no combo band. ',
				apiError: result.error
			};
			return;
		}

		const { data, meta } = result.data;
		combos = configuredCombos(data);
		combosNotice =
			meta.total > combos.length
				? { text: `Shows the first ${combos.length} of ${meta.total} combos.` }
				: null;
	}

	// One sentence about what the drawing left out, whichever half left it out: an operator counting nodes
	// on screen against a list they know should not have to work out which read they are looking at.
	const nodeNotices = $derived([providersNotice, combosNotice].filter((notice) => notice !== null));

	onMount(() => {
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

<section
	class="flex flex-col gap-3 rounded-[var(--radius-md)] border border-[var(--color-border)] p-3"
>
	<!-- One header bar over the drawing: the state readout, the sentence that explains it, and the two
	     controls that govern the stream. The status is a marker and a label, not a control-shaped box. -->
	<div class="flex flex-wrap items-center gap-3">
		<span
			class={`h-5 w-[3px] shrink-0 rounded-[var(--radius-full)] ${STATUS_MARKER[report.status]}`}
		></span>
		<span role="status" aria-live="polite" class="text-sm font-medium"
			>{streamLabel(report.status)}</span
		>
		<p class="w-full min-w-0 text-sm text-[var(--color-text-muted)] sm:w-auto sm:flex-1">
			{statusLine}
		</p>

		<Button
			variant="toggle"
			aria-pressed={paused}
			onclick={() => (paused ? controller?.resume() : controller?.pause())}
		>
			{paused ? 'Resume live updates' : 'Pause live updates'}
		</Button>

		{#if report.status === 'unavailable'}
			<Button variant="outline" onclick={() => controller?.retry()}>Try again</Button>
		{/if}
	</div>

	{#if nodeNotices.length > 0}
		<div role="status" class="flex flex-col gap-1 text-sm text-[var(--color-text-muted)]">
			{#each nodeNotices as notice, index (index)}
				<p>
					<ApiErrorText
						error={notice.apiError ?? notice.text}
						prefix={notice.apiError ? notice.text : ''}
					/>
				</p>
			{/each}
		</div>
	{/if}

	<!-- `live` is what the drawing needs to move: motion says "happening now", so it is reserved for a
	     connection frames are arriving on. A paused or dropped stream leaves the last known state on
	     screen in colour and stops every moving part. -->
	<UsageTopology
		providers={drawingProviders}
		{combos}
		{active}
		{last}
		{error}
		live={report.status === 'live'}
	/>

	<!-- Rendered only when the frame carries finished requests. While none has, the screen's statement of
	     that is the drawing's colours and the status readout, not a sentence (owner's correction,
	     2026-09-23). -->
	<!-- The live facts and the request lists sit together in one tab component at the bottom of the panel,
	     below the drawing, so the page reads Node first and the detail after it. `now` rides along because
	     the in-flight rows state how long each request has been held, and that figure has to age on the
	     same clock that ages the staleness guard rather than on a second clock of its own. -->
	<UsageLiveTabs {facts} {active} {recent} {providerName} {now} />
</section>
