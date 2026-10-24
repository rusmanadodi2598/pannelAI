<script lang="ts">
	// Copy control (SPEC-UI §6.12, §6.10): the value and label belong to the caller, and the outcome
	// is always reported. The async clipboard needs a secure context, so the selection fallback covers
	// the panel's usual http origin. A modal dialog inerts the page, so its scratch field must live in
	// the open dialog and sit off screen; display:none cannot be selected. `oncopied` fires after a
	// real write, which is how the one-time key modal unlocks dismissal. A caller that asks for
	// `glyphOutcome` moves the sentence into the live region and draws the answer on the glyph, so an
	// anchored icon never shifts when a word appears beside it.
	import { CONTROL_ICONS } from '$lib/icons';
	import { Button } from '$lib/primitives/button/index.js';

	const DEFAULT_LABEL = 'Copy';
	const COPIED = 'Copied.';
	const FAILED = 'Copy failed. Select the text and copy it.';

	const CopyIcon = CONTROL_ICONS.copy.icon;
	const CopiedIcon = CONTROL_ICONS.copyDone.icon;
	const FailedIcon = CONTROL_ICONS.copyFailed.icon;

	let {
		value,
		label = DEFAULT_LABEL,
		ariaLabel,
		iconOnly = false,
		glyphOutcome = false,
		oncopied,
		touchTarget = false
	}: {
		value: string;
		label?: string;
		/** A more specific name for a control repeated on a page, while its visible label stays short. */
		ariaLabel?: string;
		iconOnly?: boolean;
		/** Draws the outcome on the glyph and leaves the sentence to the live region. */
		glyphOutcome?: boolean;
		oncopied?: () => void;
		touchTarget?: boolean;
	} = $props();

	let state = $state<'idle' | 'copied' | 'failed'>('idle');

	/** The async clipboard API, when the context is secure enough to have one. */
	async function writeModern(text: string): Promise<boolean> {
		if (typeof navigator === 'undefined' || !navigator.clipboard) return false;
		try {
			await navigator.clipboard.writeText(text);
			return true;
		} catch {
			return false;
		}
	}

	/**
	 * Selection-based copy for an origin without the async API. `execCommand` is deprecated and remains
	 * the only path on an insecure origin.
	 */
	function writeLegacy(text: string): boolean {
		const previous = document.activeElement;
		const area = document.createElement('textarea');
		area.value = text;
		area.setAttribute('readonly', '');
		area.style.position = 'fixed';
		area.style.top = '0';
		area.style.opacity = '0';

		const dialogs = document.querySelectorAll('dialog[open]');
		(dialogs[dialogs.length - 1] ?? document.body).appendChild(area);

		try {
			area.focus();
			area.select();
			area.setSelectionRange(0, text.length);

			// `execCommand` answers true even when focus failed, so trust it only after the selection landed.
			if (document.activeElement !== area) return false;
			return document.execCommand('copy');
		} catch {
			return false;
		} finally {
			area.remove();
			// Removing the scratch field drops focus to the body, so restore the pressed control.
			if (previous instanceof HTMLElement && previous.isConnected) previous.focus();
		}
	}

	async function write(): Promise<void> {
		if ((await writeModern(value)) || writeLegacy(value)) {
			state = 'copied';
			oncopied?.();
			return;
		}

		state = 'failed';
	}
</script>

<span class="inline-flex items-center gap-2">
	{#if iconOnly}
		<Button
			variant="outline"
			size={touchTarget ? 'icon' : 'icon-sm'}
			aria-label={ariaLabel ?? label}
			title={ariaLabel ?? label}
			onclick={() => void write()}
		>
			{#if glyphOutcome && state === 'copied'}
				<CopiedIcon class="size-4 text-[var(--color-ok)]" aria-hidden="true" />
			{:else if glyphOutcome && state === 'failed'}
				<FailedIcon class="size-4 text-[var(--color-danger)]" aria-hidden="true" />
			{:else}
				<CopyIcon class="size-4" aria-hidden="true" />
			{/if}
		</Button>
	{:else}
		<Button
			variant="outline"
			size="sm"
			class={touchTarget ? 'min-h-11' : undefined}
			aria-label={ariaLabel}
			onclick={() => void write()}
		>
			<CopyIcon class="size-3.5" aria-hidden="true" />
			{label}
		</Button>
	{/if}

	<span
		class={iconOnly && glyphOutcome ? 'sr-only' : 'text-xs text-[var(--color-text-muted)]'}
		role="status"
		aria-live="polite"
	>
		{#if state === 'copied'}
			{COPIED}
		{:else if state === 'failed'}
			{FAILED}
		{/if}
	</span>
</span>
