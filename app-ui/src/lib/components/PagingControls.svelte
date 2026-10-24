<script lang="ts">
	// The Previous / Page n of m / Next band every paged list renders (SPEC-UI §6.1).
	// One band, one shape, so a paged list looks the same everywhere.
	import { CONTROL_ICONS } from '$lib/icons';
	import { Button } from '$lib/primitives/button/index.js';

	const PreviousIcon = CONTROL_ICONS.previous.icon;
	const NextIcon = CONTROL_ICONS.next.icon;

	type Props = {
		page: number;
		lastPage: number;
		onpage: (page: number) => void;
	};

	let { page, lastPage, onpage }: Props = $props();
</script>

<div class="flex flex-wrap items-center gap-3 text-sm">
	<Button variant="ghost" class="underline" disabled={page <= 1} onclick={() => onpage(page - 1)}>
		<PreviousIcon class="size-4" aria-hidden="true" />
		Previous
	</Button>
	<span class="text-[var(--color-text-muted)]">Page {page} of {lastPage}</span>
	<Button
		variant="ghost"
		class="underline"
		disabled={page >= lastPage}
		onclick={() => onpage(page + 1)}
	>
		Next
		<NextIcon class="size-4" aria-hidden="true" />
	</Button>
</div>
