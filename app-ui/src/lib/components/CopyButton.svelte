<script lang="ts">
	// Copy control (docs/SPEC-UI/001-SPEC-UI.md §6.12, §6.10).
	//
	// The value is the caller's, so the placeholder rule lives with the text that carries it and this
	// control never composes a credential. The label is a prop because what is copied differs by screen:
	// a URL on the API Docs screen, an install line on the Skills screen.
	//
	// The three sentences below belong to the control rather than to a screen, which is why they live
	// here: they describe a clipboard write failing or succeeding, and every caller reads the same either
	// way. They moved out of `strings/api-docs.ts` when Skills became the second caller.
	//
	// A clipboard write can fail: the API is absent outside a secure context, and a browser can refuse
	// one. The control reports the outcome rather than assuming success, and every caller keeps its text
	// on screen, so a failed copy is still a value the operator can select.
	const DEFAULT_LABEL = 'Copy';
	const COPIED = 'Copied.';
	const FAILED = 'Copy failed. Select the text and copy it.';

	let { value, label = DEFAULT_LABEL }: { value: string; label?: string } = $props();

	let state = $state<'idle' | 'copied' | 'failed'>('idle');

	async function write(): Promise<void> {
		try {
			await navigator.clipboard.writeText(value);
			state = 'copied';
		} catch {
			state = 'failed';
		}
	}
</script>

<span class="inline-flex items-center gap-2">
	<button
		type="button"
		class="min-h-9 rounded-[var(--radius-sm)] border border-[var(--color-border)] px-2.5 text-xs"
		onclick={() => void write()}>{label}</button
	>

	<span class="text-xs text-[var(--color-text-muted)]" role="status" aria-live="polite">
		{#if state === 'copied'}
			{COPIED}
		{:else if state === 'failed'}
			{FAILED}
		{/if}
	</span>
</span>
