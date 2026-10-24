<script lang="ts">
	// Explicit refresh control every list carries (SPEC-UI §8.6.2), shared so its name, styling, and
	// click rule live once. It stays enabled during a read and drops a second click; a disabled gate
	// reads as broken. The label and the screen's states report the read, with no spinner or stamp,
	// and its colours come from the shared button (DESIGN §3.5).
	import { CONTROL_ICONS } from '$lib/icons';
	import { Button } from '$lib/primitives/button/index.js';

	const RefreshIcon = CONTROL_ICONS.refresh.icon;

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
	<Button type="button" variant="outline" aria-busy={busy} onclick={run}>
		<RefreshIcon class="size-4" aria-hidden="true" />
		{busy ? 'Refreshing' : 'Refresh now'}
	</Button>
</div>
