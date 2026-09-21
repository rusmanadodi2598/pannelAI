<script lang="ts">
	// The explicit refresh control every list view carries (docs/SPEC-UI/001-SPEC-UI.md §8.6.2: "operators
	// distrust auto-refresh they cannot trigger").
	//
	// One component rather than a button written out on each screen, so the accessible name, the styling,
	// and the rule about clicks during a read have a single home.
	//
	// The rule: the control stays enabled while a read is in flight, and this guard drops a second click.
	// That is the decision `/quota` records for its own control, and it is the right one here for the same
	// reason: a disabled gate swallows the click, which reads as a broken button. What changes instead is
	// the label, so the click is acknowledged, and the screen's own loading state is what shows the read
	// happening. A failure is not swallowed either: the screen renders its error state, which is where the
	// operator reads why.
	//
	// It is a plain button on purpose. There is no spinner and no "last read" stamp, because the screens
	// already state both through their loading and error states, and a stamp written here could not tell a
	// read that succeeded from one that failed.
	let { onrefresh }: { onrefresh: () => void | Promise<void> } = $props();

	let busy = $state(false);

	async function run(): Promise<void> {
		if (busy) return;

		busy = true;
		try {
			await onrefresh();
		} finally {
			busy = false;
		}
	}
</script>

<!-- The row the button sits in is part of the control: a bare button in a column would stretch to the
     full width of the screen, and every caller would have to repeat the same wrapper to stop it. -->
<div class="flex flex-wrap items-center gap-3">
	<button
		type="button"
		class="min-h-11 rounded-[var(--radius-sm)] border border-[var(--color-border)] px-3 text-sm"
		onclick={run}>{busy ? 'Refreshing' : 'Refresh now'}</button
	>
</div>
