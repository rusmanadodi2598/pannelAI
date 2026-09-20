<script lang="ts">
	// The two notes under the token saver sections (docs/SPEC-UI/001-SPEC-UI.md §6.7).
	//
	// The native engine is a reserved area, not a control: §6.7 asks for a label naming the later spec,
	// and a disabled toggle would be a control an operator keeps trying. The per-request bypass is a
	// header a client author types, so it ships as a copy control with the exact value.
	import { ClipboardCopy } from '@lucide/svelte';

	const BYPASS_HEADER = 'X-Token-Saver: off';

	let copied = $state(false);
	let copyFailed = $state(false);

	async function copyBypass(): Promise<void> {
		copyFailed = false;
		try {
			await navigator.clipboard.writeText(BYPASS_HEADER);
			copied = true;
			setTimeout(() => (copied = false), 2000);
		} catch {
			// Clipboard access can be denied (an insecure origin, or a browser policy). Saying so beats a
			// control that appears to work.
			copyFailed = true;
			setTimeout(() => (copyFailed = false), 4000);
		}
	}
</script>

<div class="flex flex-col gap-4">
	<div
		class="flex flex-col gap-1 rounded-[var(--radius-md)] border border-[var(--color-border)] p-4"
	>
		<div class="flex items-center gap-2">
			<h2 class="text-sm font-semibold">Native engine</h2>
			<span
				class="rounded-[var(--radius-sm)] border border-[var(--color-border)] px-2 py-0.5 text-xs text-[var(--color-text-muted)]"
				>Planned</span
			>
		</div>
		<p class="text-sm text-[var(--color-text-muted)]">
			The gateway's own compression pass, specified in 002-TOKEN-SAVER. Nothing on this screen
			configures it yet: RTK, Headroom, and Ponytail are what runs today.
		</p>
	</div>

	<div
		class="flex flex-col gap-2 rounded-[var(--radius-md)] border border-[var(--color-border)] p-4"
	>
		<h2 class="text-sm font-semibold">Skip the savers for one request</h2>
		<p class="text-sm text-[var(--color-text-muted)]">
			A client that sends this header bypasses every saver for that request alone. Nothing is
			changed here; the header travels with the request.
		</p>
		<div class="flex flex-wrap items-center gap-3">
			<code class="rounded-[var(--radius-sm)] bg-[var(--color-surface-2)] px-2 py-1 text-xs"
				>{BYPASS_HEADER}</code
			>
			<button
				type="button"
				class="inline-flex min-h-11 items-center gap-2 rounded-[var(--radius-sm)] border border-[var(--color-border)] px-3 text-sm hover:bg-[var(--color-surface-2)]"
				onclick={copyBypass}
			>
				<ClipboardCopy class="size-4" aria-hidden="true" />
				{copied ? 'Copied' : 'Copy header'}
			</button>
			<span class="sr-only" role="status" aria-live="polite">
				{copied ? 'Header copied to the clipboard.' : ''}
				{copyFailed
					? 'The browser refused clipboard access. Select the header and copy it manually.'
					: ''}
			</span>
		</div>
	</div>
</div>
