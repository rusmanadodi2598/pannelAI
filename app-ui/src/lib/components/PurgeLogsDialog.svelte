<script lang="ts">
	// The purge confirmation dialog for Logs (docs/SPEC-UI/001-SPEC-UI.md §6.11, §8.5).
	//
	// Log purge is the screen's destructive action. The API purges by retention and reports the count only
	// after the delete, so the panel cannot name the affected rows beforehand (§14 Q6). The honest response
	// is a typed confirmation on every purge rather than on a count the panel cannot know, and this dialog
	// says what the delete actually does: it removes request logs older than the retention window, and it
	// leaves the usage records alone.
	import Modal from '$lib/components/Modal.svelte';
	import { purgeConfirmed, PURGE_CONFIRMATION } from '$lib/schemas/log-view';

	type Props = {
		open: boolean;
		busy: boolean;
		/** An error from the write, shown inside the dialog rather than over it. */
		result: string | null;
		onclose: () => void;
		onconfirm: () => void;
	};

	let { open, busy, result, onclose, onconfirm }: Props = $props();

	let typed = $state('');
</script>

<Modal
	title="Purge captured logs"
	{open}
	onclose={() => {
		typed = '';
		onclose();
	}}
>
	<div class="flex flex-col gap-4">
		<p>
			This deletes every stored request log older than the retention window set in Settings. The
			gateway reports how many rows it removed, and the removal cannot be undone.
		</p>
		<p class="text-[var(--color-text-muted)]">
			Usage records are not affected: they are a separate table and are kept for the accounting
			screens.
		</p>

		<label class="flex flex-col gap-1 text-sm">
			<span>Type <span class="font-medium">{PURGE_CONFIRMATION}</span> to confirm.</span>
			<input
				bind:value={typed}
				class="min-h-11 rounded-[var(--radius-sm)] border border-[var(--color-border)] bg-[var(--color-surface)] px-2 text-sm"
			/>
		</label>

		{#if result}
			<p class="text-sm text-[var(--color-danger)]" role="alert">{result}</p>
		{/if}
	</div>

	{#snippet footer()}
		<button
			type="button"
			class="min-h-11 rounded-[var(--radius-sm)] border border-[var(--color-border)] px-3 text-sm"
			onclick={() => {
				typed = '';
				onclose();
			}}>Keep the logs</button
		>
		<button
			type="button"
			disabled={!purgeConfirmed(typed) || busy}
			class="min-h-11 rounded-[var(--radius-sm)] bg-[var(--color-danger)] px-3 text-sm font-medium text-white disabled:opacity-50"
			onclick={onconfirm}>Purge logs</button
		>
	{/snippet}
</Modal>
