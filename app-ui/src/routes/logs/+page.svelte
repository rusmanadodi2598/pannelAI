<script lang="ts">
	// Logs (docs/SPEC-UI/001-SPEC-UI.md §6.11).
	//
	// The Requests tab is the only tab here. Console lives at `/console-log` on its own: console output is
	// read while a request is being debugged, so a tab behind the Requests list would make it a
	// second-class view. §6.11 keeps a pointer to that home, which is the link below rather than an empty
	// tab.
	import { resolve } from '$app/paths';
	import { ScrollText } from '@lucide/svelte';
	import LogsRequestsTab from '$lib/components/LogsRequestsTab.svelte';
</script>

<section class="flex flex-col gap-5">
	<div class="flex flex-col gap-1">
		<h1 class="text-lg font-semibold tracking-tight">Logs</h1>
		<p class="text-sm text-[var(--color-text-muted)]">What the gateway recorded, newest first.</p>
	</div>

	<a href={resolve('/console-log')} class="inline-flex min-h-11 items-center gap-2 text-sm">
		<ScrollText class="size-4" aria-hidden="true" />
		Console log
	</a>

	<LogsRequestsTab />
</section>
