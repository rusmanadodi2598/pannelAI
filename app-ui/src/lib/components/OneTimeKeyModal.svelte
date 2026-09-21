<script lang="ts">
	// One-time display of a newly created gateway key.
	//
	// The API returns the plaintext once and keeps only a hash, so this modal is not dismissible by
	// accident: the close control appears only after the operator ticks the acknowledgement, and the
	// value is never written to storage.
	//
	// The copy control is the shared one, which is what makes a refused clipboard write visible: it
	// reports "Copied." or the failure sentence, and the key stays on screen to select. A copy that
	// succeeded also unlocks dismissal, because at that point the key is in the operator's clipboard
	// rather than only on this screen (SPEC-UI §6.2). A refused copy does not: closing on it would lose
	// the one value the gateway will never show again.
	import CopyButton from '$lib/components/CopyButton.svelte';
	import Modal from '$lib/components/Modal.svelte';
	import type { CreatedGatewayKey } from '$lib/schemas/gateway-key';

	type Props = {
		created: CreatedGatewayKey | null;
		onclose: () => void;
	};

	let { created, onclose }: Props = $props();

	let acknowledged = $state(false);
	let copied = $state(false);

	// A new key resets the gate, so the previous acknowledgement cannot carry over.
	$effect(() => {
		if (created) {
			acknowledged = false;
			copied = false;
		}
	});
</script>

<Modal
	title="Copy this key now"
	open={created !== null}
	dismissible={acknowledged || copied}
	{onclose}
>
	{#if created}
		<p class="text-sm text-[var(--color-text-muted)]">
			This key is shown once. Store it now, because the gateway keeps only a hash of it.
		</p>

		<div class="mt-3 flex flex-wrap items-center gap-2">
			<code
				class="min-w-0 flex-1 truncate rounded-[var(--radius-sm)] bg-[var(--color-surface-2)] px-2 py-2 font-mono text-xs"
				>{created.plaintext_key}</code
			>
			<CopyButton value={created.plaintext_key} oncopied={() => (copied = true)} />
		</div>

		<label class="mt-4 flex items-center gap-2 text-sm">
			<input type="checkbox" bind:checked={acknowledged} />
			<span>I have stored this key</span>
		</label>
	{/if}

	{#snippet footer()}
		<button
			type="button"
			class="min-h-11 rounded-[var(--radius-sm)] bg-[var(--color-accent)] px-3 text-sm font-medium text-[var(--color-accent-text)] disabled:opacity-60"
			disabled={!acknowledged}
			onclick={onclose}>Done</button
		>
	{/snippet}
</Modal>
