<script lang="ts">
	// The two notes under the token saver sections (docs/SPEC-UI/001-SPEC-UI.md §6.7).
	//
	// The native engine is a reserved area, not a control: §6.7 asks for a label naming the later spec,
	// and a disabled toggle would be a control an operator keeps trying. The per-request bypass is a
	// header a client author types, so it ships as a copy control with the exact value, and it takes the
	// shared one because the panel is usually reached on an insecure origin where the async clipboard API
	// does not exist at all.
	import CopyButton from '$lib/components/CopyButton.svelte';

	const BYPASS_HEADER = 'X-Token-Saver: off';
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
			Not configurable yet. RTK, Headroom, and Ponytail run today.
		</p>
	</div>

	<div
		class="flex flex-col gap-2 rounded-[var(--radius-md)] border border-[var(--color-border)] p-4"
	>
		<h2 class="text-sm font-semibold">Skip the savers for one request</h2>
		<p class="text-sm text-[var(--color-text-muted)]">
			It applies per request, not to the saved settings.
		</p>
		<div class="flex flex-wrap items-center gap-3">
			<code class="rounded-[var(--radius-sm)] bg-[var(--color-surface-2)] px-2 py-1 text-xs"
				>{BYPASS_HEADER}</code
			>
			<CopyButton value={BYPASS_HEADER} label="Copy header" touchTarget />
		</div>
	</div>
</div>
