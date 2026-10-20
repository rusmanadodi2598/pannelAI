<script lang="ts">
	// The clear confirmation for the console screen (docs/SPEC-UI/001-SPEC-UI.md §6.14).
	//
	// The DELETE empties the ring on the server, for every panel open on it, and there is no undo. §6.14
	// asks the confirmation to name that consequence rather than let the control read like a view reset.
	// No typed guard: a purge deletes retained rows the operator cannot preview, this empties a buffer a
	// restart would have emptied anyway.
	import Modal from '$lib/components/Modal.svelte';
	import ApiErrorText from '$lib/components/ApiErrorText.svelte';
	import { Button } from '$lib/primitives/button/index.js';
	import type { ApiError } from '$lib/api/errors';

	type Props = {
		open: boolean;
		busy: boolean;
		/** An error from the write, shown inside the dialog rather than over it. */
		result: ApiError | null;
		onclose: () => void;
		onconfirm: () => void;
	};

	let { open, busy, result, onclose, onconfirm }: Props = $props();
</script>

<Modal title="Clear console buffer" {open} {onclose}>
	<div class="flex flex-col gap-4">
		<p>
			Deletes the buffered output on the gateway. Every panel open on this screen loses the same
			lines. This cannot be undone.
		</p>
		<p class="text-[var(--color-text-muted)]">
			Logging keeps running, so new lines arrive after the clear.
		</p>

		{#if result}
			<p class="text-sm text-[var(--color-danger)]" role="alert">
				<ApiErrorText error={result} />
			</p>
		{/if}
	</div>

	{#snippet footer()}
		<Button variant="outline" onclick={onclose}>Keep the buffer</Button>
		<Button variant="destructive" disabled={busy} aria-busy={busy} onclick={onconfirm}
			>Clear buffer</Button
		>
	{/snippet}
</Modal>
