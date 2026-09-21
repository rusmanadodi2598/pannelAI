<script lang="ts">
	// Copy control (docs/SPEC-UI/001-SPEC-UI.md §6.12).
	//
	// §6.12 requires a copy control for the base URL and for a curl example. The value is the caller's,
	// so the placeholder rule lives with the text that carries it and this control never composes a
	// credential.
	//
	// A clipboard write can fail: the API is absent outside a secure context, and a browser can refuse
	// one. The control reports the outcome rather than assuming success, and every caller keeps its text
	// on screen, so a failed copy is still a value the operator can select.
	import { API_DOCS_COPY as copy } from '$lib/strings/api-docs';

	let { value, label = copy.clipboard.label }: { value: string; label?: string } = $props();

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
			{copy.clipboard.copied}
		{:else if state === 'failed'}
			{copy.clipboard.failed}
		{/if}
	</span>
</span>
